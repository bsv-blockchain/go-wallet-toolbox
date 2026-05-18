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
VECTOR_PATHS=(
  "vectors/sync/brc40-user-state.json"
)

# Resolve ref → full commit SHA so the pin is immutable.
SHA="$(curl -sSL \
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
  curl -fsSL "$url" -o "$dest"
done

cat > "$SOURCE_FILE" <<EOF
# Upstream conformance vector pin.
# Update via ./conformance/scripts/refresh-vectors.sh
upstream_repo=${UPSTREAM_REPO}
upstream_sha=${SHA}
upstream_ref=${REF}
fetched_at=$(date -u +%Y-%m-%d)
EOF

echo "Done. Re-run: go test ./pkg/internal/storage/repo/syncrepo/... -v"
