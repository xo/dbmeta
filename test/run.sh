#!/bin/bash
# Runs the integration tests against every supported PostgreSQL release.
#
# D24 keeps this off CI, which tests the latest release only. This is the
# matrix that D40 requires before a release, and its output is what the support
# table in README.md must agree with.
#
# Usage: ./run.sh [release ...]

set -u

RELEASES=("${@:-9.6 10 11 12 13 14 15 16 17 18}")
read -ra RELEASES <<< "${RELEASES[*]}"
PORT=55432
FAILED=()

for rel in "${RELEASES[@]}"; do
  name="dbmeta-pg${rel//./}"
  printf '=== PostgreSQL %s ===\n' "$rel"
  podman rm -f "$name" >/dev/null 2>&1
  if ! podman run -d --rm --name "$name" \
      -e POSTGRES_PASSWORD=P4ssw0rd -p "$PORT:5432" \
      "docker.io/library/postgres:$rel" >/dev/null 2>&1; then
    echo "  could not start the image, skipping"
    FAILED+=("$rel: no image")
    continue
  fi
  # pg_isready reports ready during the bootstrap phase, before the server
  # restarts to accept network connections, so wait for a real connection
  for _ in $(seq 1 90); do
    podman exec "$name" psql -U postgres -h 127.0.0.1 -c 'SELECT 1' >/dev/null 2>&1 && break
    sleep 1
  done
  if DBMETA_POSTGRES="postgres://postgres:P4ssw0rd@localhost:$PORT/postgres?sslmode=disable" \
      go test -count=1 ./...; then
    echo "  passed"
  else
    FAILED+=("$rel")
  fi
  podman rm -f "$name" >/dev/null 2>&1
done

echo
if [ ${#FAILED[@]} -eq 0 ]; then
  echo "every release passed: ${RELEASES[*]}"
  exit 0
fi
echo "failed: ${FAILED[*]}"
exit 1
