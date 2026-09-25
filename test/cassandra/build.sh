#!/bin/bash
# Builds the Cassandra images the tests run against.
#
# The Apache image cannot be configured from the outside for what dbmeta needs
# to read, so every release is rebuilt with the settings baked in. See the
# Containerfile beside this for which settings and why.
#
# Usage:
#   ./cassandra/build.sh            every release container/cassandra.go names
#   ./cassandra/build.sh 5.0 3.11   only those
#
# It produces localhost/dbmeta/cassandra:<release>, which is what
# container/cassandra.go names. Each build pulls the Apache image and edits one
# file, so it takes about a minute per release and needs no downloads by hand.
#
# Nothing here is pushed anywhere. These are local images for local tests, and
# CI builds its own from the same file.

set -u

RUNNER="${DBMETA_RUNNER:-podman}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v "$RUNNER" >/dev/null 2>&1; then
  echo "$RUNNER is not on the path. Set DBMETA_RUNNER to the one you have."
  exit 1
fi

RELEASES=("$@")
if [ ${#RELEASES[@]} -eq 0 ]; then
  # The one list, read the same way test/run.sh reads it.
  mapfile -t RELEASES < <(cd "$HERE/.." && go run ./tool/servers 2>/dev/null \
    | awk -F'\t' '$1 ~ /^cassandra-/ { sub(/^cassandra-/, "", $1); print $1 }')
fi

if [ ${#RELEASES[@]} -eq 0 ]; then
  echo "no releases to build, and none named on the command line"
  exit 1
fi

echo "building: ${RELEASES[*]}"
failed=()
for release in "${RELEASES[@]}"; do
  tag="localhost/dbmeta/cassandra:$release"
  echo "=== $tag ==="
  if (set -x; "$RUNNER" build \
      --build-arg "RELEASE=$release" \
      --tag "$tag" \
      --file "$HERE/Containerfile" \
      "$HERE"); then
    echo "  built $tag"
  else
    echo "  FAILED $tag"
    failed+=("$release")
  fi
done

if [ ${#failed[@]} -ne 0 ]; then
  echo
  echo "failed: ${failed[*]}"
  exit 1
fi
echo
echo "built every release: ${RELEASES[*]}"
