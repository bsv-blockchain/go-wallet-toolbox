# hsm

BRC-42 / BRC-43 key derivation and signing for the wallet toolbox, with the
root private key kept inside a Securosys HSM. The default toolbox build does
not link this module's PKCS#11 code. HSM mode is selected with `hsm.enabled`
and a binary built with `-tags pkcs11`.

## Why a separate module

`github.com/miekg/pkcs11` uses cgo. Putting it in this module, and compiling
the provider binding only with `-tags pkcs11`, keeps `go test` of
`go-wallet-toolbox` free of cgo and of the Securosys provider. The toolbox
talks to a small interface in `pkg/keyderiver`. The software key path is
unchanged and remains the default.

go-sdk cannot host this yet. `wallet.KeyDeriver` is a concrete struct that
returns derived private keys, and `wallet.ProtoWallet` stores that concrete
type and reads the unexported root scalar (including for the BRC-69 Schnorr
linkage proof). See the proposal at the bottom.

## What the token does

Validated on the Securosys developer HSM Grimsel (firmware RX-3.4.0-T,
PKCS#11 provider 2.8.2):

1. `CKM_ECDH1_DERIVE_COMPRESSED_POINT` (`0x80000E12`) with
   `CK_ECDH1_DERIVE_PARAMS{kdf=CKD_NULL}` and a 33-byte compressed
   counterparty point. The secret is a session object:
   `CKO_SECRET_KEY`, `CKK_GENERIC_SECRET`, `TOKEN=false`, `SENSITIVE=true`,
   `EXTRACTABLE=false`, `SIGN=true`, `VALUE_LEN=33`. That value is the
   33-byte SEC1 shared point S. BRC-69 linkage reveal uses the same
   mechanism with `EXTRACTABLE=true` and `SENSITIVE=false`.
2. `CKM_SHA256_HMAC` of the invoice under S, inside the token. The 32-byte
   digest `h` is returned to the caller. That is a known gap: Securosys may
   later offer a handle so `h` never leaves the token. `h` is wiped after
   the point addition.
3. `CKM_EC_SCALAR_CHILD_DERIVE` (`0x80000E11`) with
   `CK_EC_SCALAR_CHILD_DERIVE_PARAMS{pScalar=h, ulScalarLen=32}`. Standard
   `C_DeriveKey` returns `CKR_MECHANISM_INVALID`. The call goes through the
   vendor function `C_DeriveKeyPair` (see `pkcs11/primus.go`). The child
   private key is `TOKEN=false`, `SENSITIVE=true`, `EXTRACTABLE=false`,
   `SIGN=true`, `DERIVE=true`. `DERIVE` is additional to the Grimsel signing
   template so BRC-43 can ECDH with the child without exporting it.
4. `CKM_ECDSA` over the 32-byte digest. `r||s` is converted to low-S DER.

Public keys are `base + h*G` in software (`base` is the root point or the
counterparty point) and cached. Remote Grimsel latencies were about 205 ms
ECDH, 430 ms HMAC, 235 ms derive, 407 ms sign. `session_pool_size` keeps
several logged-in sessions so those round trips can overlap. Derived public
keys are cached; scalars are not.

## Configuration

Toolbox (`infra-config.yaml` or `INFRA_*`):

```yaml
hsm:
  enabled: true
  library_path: /usr/local/primus/lib/libprimusP11.so
  pin_file: /run/secrets/pkcs11.pin
  key_label: server-root
  slot: -1                 # -1 = first slot that has a token
  session_pool_size: 4
  public_key_cache_size: 1024
```

`server_private_key` is not read when `hsm.enabled` is true. The provider
still needs its own files, pointed at by the environment (never commit them):

- `SECUROSYS_PKCS11_CONF`
- `SECUROSYS_SECRETS_CONF`

The PIN file is the user PIN and nothing else. It is not an environment
variable and it is not passed on argv.

The toolbox module does not import this module. `cmd/infra` in the toolbox
keeps the hex-key server. The HSM server is a separate command that passes
`infra.WithKeyDeriverFactory` so downstream modules that import `pkg/infra`
do not need an unpublished `hsm` version:

```sh
cd hsm/cmd/infra
go build -tags pkcs11 -o infra-hsm .
```

A toolbox binary without that factory refuses to start when `hsm.enabled` is set.

The on-token private key is secp256k1, looked up by label, with
`SENSITIVE=1`, `EXTRACTABLE=0`, `DERIVE=1`, `SIGN=1`.

## Tests

Unit tests use `NewSoftwareBackend` and check derive-public-key, sign/verify,
symmetric key, HMAC, encrypt/decrypt, and linkage reveal against go-sdk.
No hardware and no cgo:

```sh
cd hsm && go test ./...
```

The Grimsel integration test is behind `-tags pkcs11` and skips unless the
env vars are set. It compares the token to a software counterparty (go-sdk
can derive the child from the root *public* key plus the counterparty
private key, so the root private key is not required).

```sh
export SECUROSYS_PKCS11_CONF=/path/primus.cfg
export SECUROSYS_SECRETS_CONF=/path/.secrets.cfg
export HSM_PKCS11_LIB=/usr/local/primus/lib/libprimusP11.so
export HSM_PIN_FILE=/path/pkcs11.pin
export HSM_KEY_LABEL=server-root
export HSM_SLOT=0   # optional
cd hsm && go test -tags pkcs11 -count=1 -v ./pkcs11/ -run TestGrimselBRC42
```

## Security

- The root scalar is not returned, logged, or written to a session object
  that is extractable.
- The PIN is read from `pin_file`, passed to `C_Login`, and the buffer is
  wiped. Go retains the temporary `string` copy until it is garbage
  collected; do not log config errors in a way that includes file contents.
  Login errors from this package do not include the PIN.
- `h` (the BRC-42 HMAC) is returned by the token between HMAC and
  `C_DeriveKeyPair`. Callers in this module wipe it. Do not log it.
- BRC-43 returns the X coordinate of a derived shared point. That coordinate
  is not the root key; the counterparty can compute the same value. It is
  still not logged.
- BRC-69 `RevealCounterpartySecret` deliberately exports the ECDH point.
  The Schnorr proof go-sdk attaches (`z = r + e*a`) needs the root scalar.
  `CKM_EC_SCALAR_CHILD_DERIVE` keeps the resulting scalar inside a key
  object, so HSM mode returns an error from
  `RevealCounterpartyKeyLinkage`. `RevealSpecificKeyLinkage` does not need
  that proof and works.
- The `C_DeriveKeyPair` shim assumes a 64-bit `CK_ULONG` (LP64), which is
  the ABI of the provider build used on Grimsel.

## go-sdk interface

`pkg/keyderiver.Deriver` is the seam the toolbox uses. go-sdk should export
the same shape and store it inside `ProtoWallet` instead of `*KeyDeriver`,
so `rootKey` is no longer read directly:

```go
type KeyDeriver interface {
    IdentityKey() *ec.PublicKey
    DerivePublicKey(protocol Protocol, keyID string, counterparty Counterparty, forSelf bool) (*ec.PublicKey, error)
    DeriveSymmetricKey(protocol Protocol, keyID string, counterparty Counterparty) (*ec.SymmetricKey, error)
    Sign(protocol Protocol, keyID string, counterparty Counterparty, digest []byte) (*ec.Signature, error)
    RevealSpecificSecret(counterparty Counterparty, protocol Protocol, keyID string) ([]byte, error)
    RevealCounterpartySecret(counterparty Counterparty) (*ec.PublicKey, error)
}
```

`Sign` replaces `DerivePrivateKey` followed by `PrivateKey.Sign`. HMAC and
AES-GCM can stay in the caller once `DeriveSymmetricKey` is available; both
already run outside the HSM in go-sdk. Linkage revelation that builds a
Schnorr proof needs one more method that does not return the root scalar,
for example `ProveCounterpartyLinkage(counterparty *ec.PublicKey) (proof []byte, linkage *ec.PublicKey, error)`.
`CachedKeyDeriver` already has an unexported interface, but that interface
still includes `DerivePrivateKey`, and `ProtoWallet` does not use it.
