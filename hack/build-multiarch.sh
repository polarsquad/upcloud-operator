#!/usr/bin/env bash
# Build and publish with an invocation-owned builder; never reuse a failed
# creation or change the caller's default Buildx selection.
set -euo pipefail

: "${IMG:?IMG is required}"
: "${PLATFORMS:?PLATFORMS is required}"
CONTAINER_TOOL=${CONTAINER_TOOL:-docker}

tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/uck-build.XXXXXXXXXX")
builder="uck-builder-$$"
created=false
cleanup() {
  status=$?
  trap - EXIT
  if "$created"; then
    "$CONTAINER_TOOL" buildx rm "$builder" || true
  fi
  rm -f "$tmpdir/Dockerfile" || true
  rmdir "$tmpdir" || true
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# Only the builder stage runs natively; TARGETOS/TARGETARCH select the output.
sed -e '1 s/\(^FROM\)/FROM --platform=${BUILDPLATFORM}/; t' \
  -e '1,// s//FROM --platform=${BUILDPLATFORM}/' Dockerfile > "$tmpdir/Dockerfile"

"$CONTAINER_TOOL" buildx create --driver docker-container --name "$builder"
created=true
"$CONTAINER_TOOL" buildx build --builder "$builder" --push \
  --platform="$PLATFORMS" --tag "$IMG" -f "$tmpdir/Dockerfile" .
