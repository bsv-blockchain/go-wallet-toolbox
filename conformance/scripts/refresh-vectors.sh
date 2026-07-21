#!/usr/bin/env bash
# Refresh vendored BRC conformance vectors from upstream ts-stack.
#
# Usage:
#   ./conformance/scripts/refresh-vectors.sh             # latest main
#   ./conformance/scripts/refresh-vectors.sh <sha|ref>   # pin to specific commit/ref

set -euo pipefail

UPSTREAM_REPO="bsv-blockchain/ts-stack"
DEFAULT_REF="main"
REF="${1:-$DEFAULT_REF}"

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SOURCE_FILE="$ROOT_DIR/SOURCE"

# Tracked vector paths — keep in sync with conformance/README.md.
# For BRC-100 we start with the highest-value files (createAction, signAction, internalize,
# list*, certificate ops, get* basics). Additional brc100/*.json files can be added here
# as coverage expands. The storage adapter contract is the single file that defines the
# /storage/v1/* HTTP remoting protocol.
VECTOR_PATHS=(
  "vectors/sync/brc40-user-state.json"
  # Storage Adapter conformance (the /storage/v1/* HTTP contract for remote storage)
  "vectors/wallet/storage/adapter-conformance.json"
  # Core BRC-100 wallet method vectors (logical method call + expected result)
  "vectors/wallet/brc100/createaction.json"
  "vectors/wallet/brc100/signaction.json"
  "vectors/wallet/brc100/internalizeaction.json"
  "vectors/wallet/brc100/listoutputs.json"
  "vectors/wallet/brc100/listactions.json"
  "vectors/wallet/brc100/provecertificate.json"
  "vectors/wallet/brc100/relinquishoutput.json"
  "vectors/wallet/brc100/getpublickey.json"
  "vectors/wallet/brc100/getnetwork.json"
)

# Resolve ref → full commit SHA so the pin is immutable.
# --proto / --proto-redir restrict both the initial request and any redirects
# to HTTPS (shell:S6506).
SHA="$(curl --proto '=https' --proto-redir '=https' -sSL \
  -H 'Accept: application/vnd.github+json' \
  "https://api.github.com/repos/${UPSTREAM_REPO}/commits/${REF}" \
  | sed -n 's/^[[:space:]]*"sha":[[:space:]]*"\([0-9a-f]\{40\}\)".*/\1/p' \
  | head -n1)"

if [[ -z "$SHA" ]]; then
  echo "ERROR: could not resolve $UPSTREAM_REPO@$REF to a commit SHA" >&2
  exit 1
fi

echo "Pinning to ${UPSTREAM_REPO}@${SHA} (ref: ${REF})"

for path in "${VECTOR_PATHS[@]}"; do
  url="https://raw.githubusercontent.com/${UPSTREAM_REPO}/${SHA}/conformance/${path}"
  dest="$ROOT_DIR/$path"
  mkdir -p "$(dirname "$dest")"
  echo "  $path"
  curl --proto '=https' --proto-redir '=https' -fsSL "$url" -o "$dest"
done

cat > "$SOURCE_FILE" <<EOF
# Upstream conformance vector pin.
# Update via ./conformance/scripts/refresh-vectors.sh
upstream_repo=${UPSTREAM_REPO}
upstream_sha=${SHA}
upstream_ref=${REF}
fetched_at=$(date -u +%Y-%m-%d)
EOF

echo "Done."
echo "Re-run relevant conformance tests:"
echo "  go test ./pkg/storage/... -run AdapterConformance -v"
echo "  go test ./pkg/wallet/... -run BRC100Conformance -v"
