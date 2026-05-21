# BRC-100 JSON-RPC conformance method list

Phase 0 task: confirm each method against the BRC-100 spec and against existing Go RPC handlers. This file is the authoritative method list for Option B.

## Action lifecycle

- [ ] `createAction`
- [ ] `signAction`
- [ ] `abortAction`
- [ ] `internalizeAction`
- [ ] `listActions`

## Outputs

- [ ] `listOutputs`
- [ ] `relinquishOutput`

## Transactions

- [ ] `listTransactions` (Go-added; ensure BRC-100 alignment)

## Certificates

- [ ] `acquireCertificate`
- [ ] `proveCertificate`
- [ ] `relinquishCertificate`
- [ ] `listCertificates`
- [ ] `discoverByIdentityKey`
- [ ] `discoverByAttributes`

## Identity / authentication

- [ ] `isAuthenticated`
- [ ] `waitForAuthentication`
- [ ] `getPublicKey`
- [ ] `revealCounterpartyKeyLinkage`
- [ ] `revealSpecificKeyLinkage`

## Cryptographic ops

- [ ] `encrypt`
- [ ] `decrypt`
- [ ] `createHmac`
- [ ] `verifyHmac`
- [ ] `createSignature`
- [ ] `verifySignature`

## Chain / network info

- [ ] `getHeight`
- [ ] `getHeaderForHeight`
- [ ] `getNetwork`
- [ ] `getVersion`

## Audit / known issues (Phase 0)

For each method below, audit:

1. Does the request shape leak internal state (e.g. surrogate IDs returned in a prior call)?
2. Does the response include any field that requires a particular storage shape to compute?
3. Is there an implicit ordering requirement that depends on storage layout?

Findings go in this file under `## Phase 0 audit findings` once Phase 0 begins.

## Phase 0 audit findings

(To be filled in during Phase 0.)
