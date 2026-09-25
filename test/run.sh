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
#   ./run.sh vms              every provisioned Windows machine
#
# It starts what it needs and removes it afterwards. It never uses a container
# somebody else started: the names and ports here are its own, so a container
# called postgres or mssql on a default port is left alone.
#
# The Windows machines are the exception, because installing one takes an hour.
# `./run.sh vms` runs against the machines vm/provision.sh has already made,
# starting any that are stopped, and says which are missing rather than
# building them.
#
# It needs podman on the path. Set DBMETA_RUNNER=docker to use docker instead.
#
# Cassandra needs its image built first, because the published one refuses
# what the queries read: run ./cassandra/build.sh. Every other product uses a
# published image and needs nothing.
#
# It also needs the machine mostly to itself. Each server wants a gigabyte or
# two, and a run with other heavy containers already up reports "not ready" for
# whichever servers lost the race, which looks exactly like a broken query and
# is not one. Stop the Oracle containers and the Windows machines first.
# Neither is a dependency of dbmeta: this script runs a command, and the Go
# package it reads the list from starts nothing.

set -u

RUNNER="${DBMETA_RUNNER:-podman}"
if ! command -v "$RUNNER" >/dev/null 2>&1; then
  echo "$RUNNER is not on the path. Set DBMETA_RUNNER to the one you have."
  exit 1
fi

# The Windows machines, which are provisioned rather than started fresh.
if [ "${1:-}" = "vms" ] || [ "${1:-}" = "vm" ]; then
  PASSED=()
  FAILED=()
  while IFS=$'\x1f' read -r name release image port viewer regkey license url file dsn; do
    printf '=== %s ===\n' "$name"
    if ! $RUNNER container exists "$name" 2>/dev/null; then
      echo "  not provisioned. Run ./vm/provision.sh $release, which takes an hour."
      FAILED+=("$name: not provisioned")
      continue
    fi
    if [ "$($RUNNER inspect --format '{{.State.Status}}' "$name" 2>/dev/null)" != "running" ]; then
      echo "  starting it"
      $RUNNER start "$name" >/dev/null 2>&1
    fi
    # A machine boots Windows before SQL Server listens, so this waits on a
    # query rather than on the port. See vm/README.md.
    if ! go run ./tool/vms -wait 10m "$release" >/dev/null 2>&1; then
      echo "  it never answered. Look at http://127.0.0.1:$viewer"
      FAILED+=("$name: never answered")
      continue
    fi
    if env DBMETA_SQLSERVER="$dsn" go test -count=1 ./...; then
      echo "  passed"
      PASSED+=("$name")
    else
      FAILED+=("$name")
    fi
  done < <(go run ./tool/vms)

  echo
  if [ ${#FAILED[@]} -eq 0 ]; then
    echo "every machine passed: ${PASSED[*]}"
    exit 0
  fi
  echo "passed: ${PASSED[*]-none}"
  echo "failed: ${FAILED[*]}"
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

  # Each command field holds its arguments separated by a unit separator
  # rather than by a space, because an argument can contain a space: the SQL
  # Server readiness command runs the query "SELECT 1". Splitting these on
  # whitespace passed a broken command and looked like a server that never
  # started. Read them into arrays and expand with "${name[@]}".
  IFS=$'\x1f' read -r -a RUN <<< "$runargs"
  IFS=$'\x1f' read -r -a READY <<< "$readyargs"
  IFS=$'\x1f' read -r -a REMOVE <<< "$removeargs"

  # remove a container left behind by a run that was interrupted
  "$RUNNER" "${REMOVE[@]}" >/dev/null 2>&1

  if ! "$RUNNER" "${RUN[@]}" >/dev/null 2>&1; then
    echo "  could not start ${name}, skipping"
    # Cassandra is the one product whose image this repository builds. The
    # published one refuses a user defined function, a materialized view and
    # a role, so the fixture cannot build and the failure reads as a missing
    # image. Say what to do rather than leave it at that.
    case "$name" in
      cassandra-*) echo "  build it first: ./cassandra/build.sh" ;;
    esac
    FAILED+=("$name: no image")
    continue
  fi

  # Wait for a real connection. Several of these images start, bootstrap a
  # data directory and restart, and a connection made in between is refused,
  # so a fixed pause is not enough and neither is the first successful check
  # on a local socket.
  ready=no
  for _ in $(seq 1 90); do
    if "$RUNNER" "${READY[@]}" >/dev/null 2>&1; then
      ready=yes
      break
    fi
    sleep 1
  done
  if [ "$ready" = no ]; then
    echo "  ${name} never became ready"
    FAILED+=("$name: not ready")
    "$RUNNER" "${REMOVE[@]}" >/dev/null 2>&1
    continue
  fi

  if env "$envvar=$dsn" go test -count=1 ./...; then
    echo "  passed"
    PASSED+=("$name")
  else
    FAILED+=("$name")
  fi

  "$RUNNER" "${REMOVE[@]}" >/dev/null 2>&1
done <<< "$SERVERS"

echo
if [ ${#FAILED[@]} -eq 0 ]; then
  echo "every release passed: ${PASSED[*]}"
  exit 0
fi
echo "passed: ${PASSED[*]}"
echo "failed: ${FAILED[*]}"
exit 1
