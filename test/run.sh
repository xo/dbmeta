#!/bin/bash
# Runs the integration tests against every database release dbmeta supports.
#
# This is the matrix D40 requires before a release. CI runs the Tested tier on
# every push and the Nightly tier once a night, and this runs whatever you ask
# for, on your own machine, against containers it starts and removes.
#
# The list of releases is not here. It lives in the Go package
# github.com/xo/dbmeta/container, and `go run ./tool/servers` prints it. One
# copy, so that this script and the CI workflow cannot disagree about which
# releases a release was tested against.
#
# Usage:
#   ./run.sh                  every release of every product
#   ./run.sh tested           the four releases CI runs on every push
#   ./run.sh nightly          the releases CI runs at night
#   ./run.sh postgres         every PostgreSQL release
#   ./run.sh mariadb mysql    both products of the mysql dialect
#   ./run.sh mariadb-11.8     one release
#
# It needs podman on the path. Set DBMETA_RUNNER=docker to use docker instead.
# Neither is a dependency of dbmeta: this script runs a command, and the Go
# package it reads the list from starts nothing.

set -u

RUNNER="${DBMETA_RUNNER:-podman}"
if ! command -v "$RUNNER" >/dev/null 2>&1; then
  echo "$RUNNER is not on the path. Set DBMETA_RUNNER to the one you have."
  exit 1
fi

SERVERS=$(go run ./tool/servers "$@") || exit 1
if [ -z "$SERVERS" ]; then
  echo "no server matches: $*"
  exit 1
fi

PASSED=()
FAILED=()

while IFS=$'\t' read -r name dsn envvar runargs readyargs removeargs; do
  printf '=== %s ===\n' "$name"

  # remove a container left behind by a run that was interrupted
  $RUNNER $removeargs >/dev/null 2>&1

  if ! $RUNNER $runargs >/dev/null 2>&1; then
    echo "  could not start ${name}, skipping"
    FAILED+=("$name: no image")
    continue
  fi

  # Wait for a real connection. Several of these images start, bootstrap a
  # data directory and restart, and a connection made in between is refused,
  # so a fixed pause is not enough and neither is the first successful check
  # on a local socket.
  ready=no
  for _ in $(seq 1 90); do
    if $RUNNER $readyargs >/dev/null 2>&1; then
      ready=yes
      break
    fi
    sleep 1
  done
  if [ "$ready" = no ]; then
    echo "  ${name} never became ready"
    FAILED+=("$name: not ready")
    $RUNNER $removeargs >/dev/null 2>&1
    continue
  fi

  if env "$envvar=$dsn" go test -count=1 ./...; then
    echo "  passed"
    PASSED+=("$name")
  else
    FAILED+=("$name")
  fi

  $RUNNER $removeargs >/dev/null 2>&1
done <<< "$SERVERS"

echo
if [ ${#FAILED[@]} -eq 0 ]; then
  echo "every release passed: ${PASSED[*]}"
  exit 0
fi
echo "passed: ${PASSED[*]}"
echo "failed: ${FAILED[*]}"
exit 1
