# CertificateType / SerialNumber short base64 wire format (TS compat)

**Issue:** [bsv-blockchain/go-wallet-toolbox#769](https://github.com/bsv-blockchain/go-wallet-toolbox/issues/769)
**Severity:** High — breaks cross-impl certificate flows with the TS SDK ecosystem (list filters, certifier issuance, storage round-trips).
**Prior attempt:** [#941](https://github.com/bsv-blockchain/go-wallet-toolbox/pull/941) (`fix/769-cert-type-serial-wire-format`) closed **unmerged**; approach below aligns with that diff and remains valid against current `origin/main`.

---

## Context for a fresh session

You are picking up a wire-format compatibility bug between Go wallet-toolbox and the TypeScript SDK (`@bsv/sdk`).

In TypeScript, certificate **types** and often **serials** are opaque `Base64String` values of arbitrary decoded length (≤32 in practice for fixed-array consumers). Real-world types such as `"CommonSource identity"` are base64-encoded **as-is** (21 bytes → `Q29tbW9uU291cmNlIGlkZW50aXR5`) with **no zero-padding**.

In Go:

- `sdk.CertificateType` / `sdk.SerialNumber` are `[32]byte` (aliases of `Bytes32Base64`).
- go-sdk `Bytes32Base64.UnmarshalJSON` requires **exactly** 32 decoded bytes.
- go-sdk `CertificateTypeFromBase64` already accepts **≤32** and zero-pads — the good reference semantic — but `CertificateType.Base64()` and every toolbox `base64.StdEncoding.EncodeToString(x[:])` re-encode the **full 32-byte array**, emitting trailing-zero pad (`…AAAAAAAAAAAAAAA=`).

Two failure modes follow:

1. **Strict decode rejects short wire values** when toolbox parsers require `len(decoded) == 32`.
2. **Re-encode corrupts identity** — storage filters, certifier HTTP bodies, and prove lookups send a different base64 string than the one the TS side registered / stored.

**Probe evidence on `origin/main` (still broken):**

```text
pkg/wdk/certificate.go:219–220   parseSerialNumber: len != 32 → error
pkg/wdk/certificate.go:237–238   parseCertificationType: len != 32 → error
pkg/wallet/internal/mapping/mapping_verifiable_certificate.go:20–21, 32–33  same strict checks
pkg/wallet/internal/actions/wallet_acquire_certificate_issuance.go:110  EncodeToString(Type[:])  // zero-pad
pkg/wallet/wallet.go:899–900, 1003, 1044  EncodeToString on Type/SerialNumber
pkg/wallet/internal/mapping/mapping_list_certificates_args.go:19
pkg/wallet/internal/mapping/mapping_relinquish_certificate_args.go:27–28
pkg/internal/validate/validate_prove_certificate_args.go:19, 26
```

Concrete example from the issue:

| Form | Base64 | Decoded length |
|------|--------|-----------------|
| TS original | `Q29tbW9uU291cmNlIGlkZW50aXR5` | 21 |
| Go naive re-encode of `[32]byte` | `Q29tbW9uU291cmNlIGlkZW50aXR5AAAAAAAAAAAAAAA=` | 32 |

Remote certifier (`https://cert.commonsource.nl`) rejects the padded form as an unknown type.

---

## Root cause

Two complementary mismatches:

### A. Decode path is stricter than go-sdk `CertificateTypeFromBase64`

Toolbox helpers decode base64 then require exact array length:

```go
// pkg/wdk/certificate.go — current main
if len(serialBytes) != len(serial) { // must be 32
    return ..., fmt.Errorf("serial bytes length: %d is not equal ...", ...)
}
```

TS (and any short type already in the DB) legitimately produces 0–32 decoded bytes. `CertificateTypeFromBase64` in go-sdk already implements the correct rule (`len > 32` error, else zero-pad copy); toolbox reimplemented the wrong rule.

### B. Encode path never trims trailing `0x00`

Every site that maps `[32]byte` → storage / HTTP / filter string uses:

```go
base64.StdEncoding.EncodeToString(certType[:]) // full 32 bytes
```

or go-sdk `CertificateType.Base64()` which does the same. After a short type has been zero-padded into `[32]byte`, re-encoding no longer equals the original wire string. That breaks:

- string-equality filters in storage (`type IN (...)`, serial lookup on prove)
- certifier request bodies that match registered type strings
- round-trip list → acquire → list identity

Storage columns and WDK wire types already use opaque `primitives.Base64String` (no fixed length) — the corruption happens at the **boundary** where SDK `[32]byte` is converted to/from those strings.

---

## Recommended fix

### 1. Shared helpers in `pkg/wdk/primitives`

Add (new file suggested: `pkg/wdk/primitives/bytes32_base64.go`, or next to `strings.go`):

```go
// EncodeBytes32Base64 encodes a fixed 32-byte value as standard base64,
// trimming trailing 0x00 so the result matches TS short Base64String forms.
// All-zero input keeps the full 32-byte encoding (distinct from empty).
func EncodeBytes32Base64(b [32]byte) string {
    trimmed := bytes.TrimRight(b[:], "\x00")
    if len(trimmed) == 0 {
        trimmed = b[:] // preserve all-zero as full 32-byte base64
    }
    return base64.StdEncoding.EncodeToString(trimmed)
}

// DecodeBytes32Base64 accepts 0–32 decoded bytes (TS short forms),
// zero-pads into [32]byte. Rejects >32 or invalid base64.
// Empty string / empty decode → zero array (symmetry with go-sdk
// StringBase64.ToArray / CertificateTypeFromBase64).
func DecodeBytes32Base64(s string) ([32]byte, error) {
    raw, err := base64.StdEncoding.DecodeString(s)
    if err != nil {
        return [32]byte{}, fmt.Errorf("invalid base64: %w", err)
    }
    if len(raw) > 32 {
        return [32]byte{}, fmt.Errorf("decoded length %d exceeds 32", len(raw))
    }
    var out [32]byte
    copy(out[:], raw)
    return out, nil
}
```

Semantics mirror go-sdk `CertificateTypeFromBase64` on decode and the issue’s suggested `TrimmedBase64` on encode. **Do not** change go-sdk from this repo — stay inside toolbox boundaries and work around.

**Locked decisions (do not re-litigate in the implementation PR):**

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Empty base64 / empty decode | Accept → zero `[32]byte` | Matches go-sdk pad-copy semantics; #941 did this |
| Encoding alphabet | `base64.StdEncoding` only (not Raw/URL) | Matches every existing toolbox call site + storage |
| All-zero array encode | Full 32-byte base64 (`AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=`) | Distinct from “empty”; avoids empty-string wire |
| Type **and** serial both trim | Yes, one rule | Consistent; serials from HMAC issuance are full 32 non-zero-biased bytes in practice |
| Copy-source | Closed #941 helpers + call-site list | Already proven shape against this codebase |

### 2. Wire helpers through every toolbox boundary

| File | Change |
|------|--------|
| `pkg/wdk/certificate.go` | `parseSerialNumber` / `parseCertificationType` → `DecodeBytes32Base64` (drop strict `len == 32`) |
| `pkg/wallet/internal/mapping/mapping_verifiable_certificate.go` | Decode type + serial via helper; cast to `wallet.CertificateType` / `SerialNumber` |
| `pkg/wallet/internal/mapping/mapping_list_certificates_args.go` | `EncodeBytes32Base64` for each type filter |
| `pkg/wallet/internal/mapping/mapping_relinquish_certificate_args.go` | Encode type + serial |
| `pkg/wallet/internal/actions/wallet_acquire_certificate_issuance.go` | Certifier request `Type` field (~L110) |
| `pkg/wallet/wallet.go` | Direct acquire insert (~L899–900); prove list serial (~L1003); prove `StringBase64` serial (~L1044) |
| `pkg/internal/validate/validate_prove_certificate_args.go` | Type + serial validation strings via encode helper (not `cert.Type.Base64()` / raw `EncodeToString`) |

**Do not rewrite** paths that already hold the original string (e.g. issuance insert that stores `p.CertTypeB64` / response serial as received). Those are correct once the request type is encoded with the trim helper.

### 3. Leave upstream go-sdk alone (residual)

`github.com/bsv-blockchain/go-sdk` `Bytes32Base64.UnmarshalJSON` still requires exactly 32 bytes on current v1.3.0. Call sites that JSON-unmarshal **directly** into `sdk.ListCertificatesArgs` / `sdk.Certificate` will still fail until go-sdk is fixed. Toolbox storage / WDK RPC already uses `primitives.Base64String` and is fine once mapping/encode is fixed.

If the BRC-100 HTTP surface unmarshals into SDK types before wallet entry, that is a **go-sdk PR** (or a thin adapter that unmarshals into string-typed DTOs first). Call that out in the implementation PR notes; it is not a blocker for the toolbox encode/decode fix, which unblocks certifier + storage string identity.

---

## Files to change (implementation PR — not this plan PR)

| Path | Role |
|------|------|
| `pkg/wdk/primitives/bytes32_base64.go` (**new**) | `EncodeBytes32Base64` / `DecodeBytes32Base64` |
| `pkg/wdk/primitives/bytes32_base64_test.go` (**new**) | unit tests for encode/decode + round-trip |
| `pkg/wdk/certificate.go` | short-type parse in `ToSDKCertificate` path |
| `pkg/wdk/certificate_test.go` (or adjacent) | short type+serial `ToSDKCertificate` |
| `pkg/wallet/internal/mapping/mapping_*.go` | list / relinquish / verifiable |
| `pkg/wallet/internal/mapping/mapping_list_certificates_args_test.go` (**new**) | short type no zero-pad |
| `pkg/wallet/internal/actions/wallet_acquire_certificate_issuance.go` | certifier request type |
| `pkg/wallet/wallet.go` | direct insert + prove serial encoding |
| `pkg/internal/validate/validate_prove_certificate_args.go` | validation encode |
| optional: wallet integration tests under `pkg/wallet/*certificate*_test.go` | acquire → list with short type |

**Out of scope for the code fix:**

- BRC-104 nonce HMAC incompatibility with TS certifiers (explicitly separate in #769; GHSA-related TS v1/v2 break).
- Changing go-sdk `Bytes32Base64` (upstream).
- Migrating already-stored **padded** base64 rows in DBs that were written by buggy encode (see Risks).

---

## Test strategy

### Unit — primitives

1. **Short type round-trip:** decode `Q29tbW9uU291cmNlIGlkZW50aXR5` (28 chars, 21 decoded bytes, `"CommonSource identity"`) → `[32]byte` with 11 trailing zeros → encode back to **exact** short string.
2. **Naive pad must differ:** `EncodeToString(full[:])` = `Q29tbW9uU291cmNlIGlkZW50aXR5AAAAAAAAAAAAAAA=` (44 chars); helper equals short string (28 chars).
3. **Full 32-byte value:** no trim of interior zeros that are not trailing; only trailing `0x00`.
4. **All-zero array:** encode yields `AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=` (full 32-byte base64, not empty).
5. **Empty base64 / empty decode:** decode `""` → zero array (**locked** above).
6. **>32 decoded bytes:** error.
7. **Invalid base64:** error.
8. **Encode output always `Base64String.Validate`-safe:** length `% 4 == 0` (StdEncoding padding).

### Unit — mapping / wdk

1. `MapListCertificatesArgs` with short type zero-padded into `CertificateType` → types slice equals original short base64.
2. `MapRelinquish…` type+serial encode trimmed.
3. `MapVerifiableCertificateToCertificate` accepts short type and short serial base64.
4. `WalletCertificate.ToSDKCertificate` accepts short type+serial stored as `primitives.Base64String`.

### Integration — wallet

1. **Short-type acquire (direct)** → insert storage with short type string → `ListCertificates` filtered by short type returns the cert.
2. **Prove lookup** by serial does not fail when SDK holds zero-padded `[32]byte` but DB has short (or vice versa after fix — both sides use trimmed encode).
3. Existing certificate suites still pass (`go test ./pkg/wallet/ -run Certificate`, `./pkg/storage/ -run Certificate`, `./pkg/wdk/`, `./pkg/wdk/primitives/`, `./pkg/wallet/internal/mapping/`, `./pkg/internal/validate/`).

### Manual / cross-impl (optional smoke)

Against a TS certifier (e.g. commonsource) with type `CommonSource identity`:

- `acquireCertificate` issuance request body `type` field equals short base64 (not padded).
- Note: may still fail later on BRC-104 HMAC — that is **not** this bug.

---

## Acceptance criteria

- [ ] `DecodeBytes32Base64` accepts 0–32 decoded bytes (including empty → zero array) and rejects `>32` / bad base64.
- [ ] `EncodeBytes32Base64` trims trailing `0x00`; all-zero keeps full 32-byte encoding.
- [ ] All toolbox sites listed above that convert `[32]byte` ↔ base64 string for type/serial use the helpers (no remaining bare `EncodeToString(typeOrSerial[:])` for those fields).
- [ ] Short TS type `Q29tbW9uU291cmNlIGlkZW50aXR5` survives list-filter mapping and `ToSDKCertificate` parse.
- [ ] Certifier issuance request encodes short type without zero-pad suffix.
- [ ] Unit + targeted wallet/storage certificate tests green.
- [ ] Implementation PR body links `Fixes #769` (this plan PR must **not** close the issue).
- [ ] Residual go-sdk `Bytes32Base64` JSON strictness documented in the implementation PR (not silently ignored).

### Completeness grep (implementation PR checklist)

After the call-site swaps, these should return **no production hits** outside the helpers/tests:

```bash
# bare re-encode of Type/SerialNumber fixed arrays (should be empty after fix)
rg -n 'EncodeToString\((args|cert|p\.Args)\.(Type|SerialNumber)\[:\]\)' pkg/

# go-sdk padded Base64() used for wire/storage of cert type (should be empty after fix)
rg -n 'cert\.Type\.Base64\(\)|args\.Type\.Base64\(\)' pkg/

# remaining strict len==32 decode of type/serial (should be empty after DecodeBytes32Base64)
rg -n 'len\((serialBytes|certTypeBytes|certBytes)\) != len\(' pkg/wdk pkg/wallet/internal/mapping
```

Positive check: helpers exist and are imported where needed:

```bash
rg -n 'EncodeBytes32Base64|DecodeBytes32Base64' pkg/
```

### End-to-end flow coverage (why each site matters)

```text
TS short type base64
        │
        ▼
  DecodeBytes32Base64  ←── parseSerialNumber / parseCertificationType (wdk)
        │                   MapVerifiableCertificateToCertificate (discover)
        ▼
   sdk.[32]byte  (zero-padded in memory)
        │
        ▼
  EncodeBytes32Base64  ←── list filter, relinquish, direct insert,
        │                   prove serial lookup, certifier issuance Type,
        │                   validate_prove type/serial Validate strings
        ▼
  storage / HTTP / filter string  (== original short base64)
```

`ListCertificates` results re-enter via `WalletCertificate.ToSDKCertificate` → decode helpers. Issuance insert that already holds `p.CertTypeB64` / response serial **as received** must keep those strings (do not re-encode).

---

## Risks / gotchas

1. **Trailing-zero serials:** Cryptographic serials (e.g. HMAC material) that happen to end in `0x00` will trim on encode. Round-trip through `[32]byte` recovers the same array; **string** identity with a previously full-padded DB row could miss until re-stored. Certificate **types** (string-like, zero-padded) are the primary production failure; serials from issuance are usually full 32 non-zero-biased bytes. Accept the trim for both type and serial for one consistent rule (matches #769 suggestion and closed #941).

2. **Already-stored padded rows:** Environments that already persisted `…AAA=` type strings from buggy encode will not match new short filters. Mitigation options (implementation decision, not required in v1 of the fix):
   - dual-read filter (`IN (short, padded)`), or
   - one-shot migration trimming trailing-zero base64 in `certificates.type` / `serial_number`, or
   - document operator re-acquire.
   Prefer documenting + dual-read only if a failing customer DB is confirmed; keep the first fix minimal.

3. **go-sdk `CertificateType.Base64()`** still pads. Never use it for wire/storage in toolbox after the fix; always `primitives.EncodeBytes32Base64`.

4. **Do not “fix” by widening go-sdk from this repo** via replace directives in the implementation PR unless maintainers explicitly want a coupled go-sdk bump.

5. **BRC-104 HMAC** remains a separate failure after type encoding is fixed — do not scope-creep.

6. **`Base64String.Validate` length rule:** toolbox requires `len(s) % 4 == 0`. `StdEncoding.EncodeToString` always produces that; short TS values that are valid StdEncoding (e.g. 28-char CommonSource type) already pass. Do not switch to RawStdEncoding.

7. **Existing test fixtures:** `pkg/internal/testabilities/certificates.go` `CreateTestCertificateType` / `SerialNumber` fill 32 random bytes — typically no long trailing-zero run, so existing suites stay green without fixture changes. Add **new** short-type cases; do not rewrite the random fixtures.

8. **Issuance serial length:** `wallet_acquire_certificate_issuance.go` still requires `len(parsedCert.SerialNumber) == NonceHMACSize` (32) for HMAC verify — that is protocol material, not a wire-format type. Do not loosen that check as part of this fix.

---

## Estimated size

**S–M** — small pure helpers + mechanical call-site swaps + focused tests. No schema migration required for the minimal fix. Prior #941 already proved the shape (~11 files: helpers + tests + mapping/wdk/wallet/validate); re-apply against current main and land with green CI.

---

## Useful cross-references

- Issue: <https://github.com/bsv-blockchain/go-wallet-toolbox/issues/769>
- Closed prior fix PR (reference only): <https://github.com/bsv-blockchain/go-wallet-toolbox/pull/941>
- go-sdk (v1.3.0 as of current toolbox `go.mod`):
  - `wallet/encoding.go` — `Bytes32Base64` strict UnmarshalJSON
  - `wallet/interfaces.go` — `CertificateTypeFromBase64` (≤32, zero-pad) and `CertificateType.Base64()` (full pad)
  - `wallet/encoding_json.go` — CertificateType/SerialNumber JSON via Bytes32Base64
- TS: `@bsv/sdk` `ListCertificatesArgs.types: Base64String[]` (no fixed length)
- Example short type: `"CommonSource identity"` → `Q29tbW9uU291cmNlIGlkZW50aXR5`
- Related-but-separate: BRC-104 HMAC / GHSA-vjpq-xx5g-qvmm (comment on #769)

---

## Implementation sketch (for the future code PR)

Commit sequence suggestion:

1. `feat(primitives): Encode/DecodeBytes32Base64 for short TS wire forms` + unit tests.
2. `fix(wdk,mapping): accept short cert type/serial on decode` (`certificate.go`, verifiable mapping).
3. `fix(wallet): trim zero-pad on type/serial encode` (list, relinquish, acquire, prove, validate).
4. `test(wallet): short certificate type acquire/list round-trip`.

Commit message pattern: `fix: CertificateType/SerialNumber short base64 TS wire compat (#769)`.
