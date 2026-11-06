## CERTIFIER SERVER: BSV Wallet Toolbox

Signs certificates for wallets using the `acquireCertificate` issuance protocol (`POST /signCertificate`, BRC-104 authenticated). Package `pkg/certifier`, runnable binary `cmd/certifier`.

### Issuance flow

- The wallet sends `clientNonce`, `type`, encrypted `fields` and `masterKeyring`.
- The server creates a `serverNonce`; the serial number is the HMAC of both nonces.
- The server wallet creates a 1-sat revocation output locked with `OP_SHA256 sha256(serialNumber) OP_EQUAL`, so the serial number unlocks it.
- The certificate is signed, stashed, and returned with the `serverNonce`.

### Certificate stash

Every issued certificate is stored by the certifier in a `CertificateStash`:

- the certificate (type, serial number, subject, certifier, revocation outpoint, signature),
- its fields as signed (encrypted values) with the subject's master keyring entry for each field,
- the revocation transaction returned by `CreateAction`, needed later to spend the revocation output.

Records can be looked up by serial number, revocation outpoint or subject.

The certificate and its fields are written in one transaction. If saving fails, `signCertificate` returns an error and no certificate is returned. The revocation output already exists at that point; its outpoint is logged with the error.

#### Storage backend

The stash is kept **separate from the certifier wallet's storage**. The wallet storage holds the certifier's own UTXOs (including the revocation outputs in the `certificate revocation` basket) and its own certificates, while the stash holds what the certifier issued to others.

`SQLStash` is the built-in implementation: GORM with its own tables, `certifier_issued_certificates` and `certifier_issued_certificate_fields`.

- `NewSQLiteStash("")` opens an in-memory SQLite database. This is the default when no stash is configured and is lost on restart.
- `NewSQLiteStash("certifier-stash.sqlite")` keeps the stash in a SQLite file.
- `NewSQLStash(db)` uses an existing `*gorm.DB` connection (e.g. Postgres).
- Any other store can be plugged in by implementing `CertificateStash` and passing it with `certifier.WithStash(...)`.

For `cmd/certifier`, set the SQLite DSN in the config file (empty means in-memory):

```yaml
stash:
  dsn: "certifier-stash.sqlite"
```
