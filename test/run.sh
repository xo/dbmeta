#!/usr/bin/env bash
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
#   ./run.sh up clickhouse-26.9    start it and leave it running
#   ./run.sh down clickhouse-26.9  remove it
#   ./run.sh up tested             start the whole Tested tier and leave it
#
# up and down exist so that nobody has to reach for the container runner by
# hand. A container started here is named <product>-<release>, which is what
# container.Server.Name returns, and D68 says every container in this project
# carries that name and is started this way. A container named something else
# is somebody improvising, and the last time that happened the machine ended
# up with a ch268 nobody could place.
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
# Cassandra's image is built rather than published, because the published one
# refuses what the queries read. This script builds it when it is missing, so
# nothing has to be done first.
#
# It also needs the machine mostly to itself. Each server wants a gigabyte or
# two, and a run with other heavy containers already up reports "not ready" for
# whichever servers lost the race, which looks exactly like a broken query and
# is not one. Stop the Oracle containers and the Windows machines first.
# Neither is a dependency of dbmeta: this script runs a command, and the Go
# package it reads the list from starts nothing.

set -u

# Run from the script's own directory, so ./test/run.sh works from the
# repository root and not only from inside test. The Go tool this reads the
# server list from is ./tool/servers relative to here.
HERE="$(cd -P "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$HERE" || exit 1

usage() {
  cat <<'EOF'
run.sh starts the databases dbmeta is tested against, runs the integration
tests against each, and removes it again. The list of releases is not here: it
lives in the Go package github.com/xo/dbmeta/container, so this script and the
CI workflow cannot disagree about what was tested.

Usage: run.sh [mode] [selector...]

Modes:
  (none)     start each server, run the tests against it, remove it
  start      start each server and leave it running, then print its DSN
  stop       stop each running server, keeping it so start resumes it
  remove     stop and delete each server
  status     list the servers that are running, with their DSN
  version    connect to each running server and print what dbmeta reads
  dsn        print the dburl style URL, running or not
  usql       connect to it with usql, which must be on the path
  --help     this

Selectors:
  (none), --all   every release of every product
  tested          the releases CI runs on every push
  nightly         the releases CI runs at night
  <product>       every release of it, such as postgres or clickhouse
  <product>-<ver> one release, such as clickhouse-26.9 or oracle-26ai
  vms             the provisioned Windows machines

Every container is named <product>-<release> and started by this script.
D68 says so, and it is why up and down exist: nothing else should be
reaching for podman by hand.

Environment:
  DBMETA_RUNNER   podman by default, set to docker to use that instead
EOF
}

case "${1:-}" in
-h | --help | help)
  usage
  exit 0
  ;;
esac

RUNNER="${DBMETA_RUNNER:-podman}"
if ! command -v "$RUNNER" >/dev/null 2>&1; then
  echo "$RUNNER is not on the path. Set DBMETA_RUNNER to the one you have."
  exit 1
fi

# The mode, which decides what happens to each server the selectors pick.
MODE=test
case "${1:-}" in
start | stop | remove | status | version | dsn | usql)
  MODE=$1
  shift
  ;;
esac

# --all is the selector that means every server, spelled so that `stop --all`
# reads the way a person says it. It is the same as naming nothing.
if [ "${1:-}" = "--all" ]; then
  shift
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

while IFS=$'\t' read -r name dsn envvar url runargs readyargs removeargs; do
  # status and version print one line per server, so a header per server
  # would be more banner than answer.
  case "$MODE" in
  status | version | dsn) ;;
  *) printf '=== %s ===\n' "$name" ;;
  esac

  # Each command field holds its arguments separated by a unit separator
  # rather than by a space, because an argument can contain a space: the SQL
  # Server readiness command runs the query "SELECT 1". Splitting these on
  # whitespace passed a broken command and looked like a server that never
  # started. Read them into arrays and expand with "${name[@]}".
  IFS=$'\x1f' read -r -a RUN <<< "$runargs"
  IFS=$'\x1f' read -r -a READY <<< "$readyargs"
  IFS=$'\x1f' read -r -a REMOVE <<< "$removeargs"

  running=no
  if [ "$("$RUNNER" inspect --format '{{.State.Status}}' "$name" 2>/dev/null)" = running ]; then
    running=yes
  fi

  case "$MODE" in
  status)
    # The URL rather than the driver DSN, because the answer to "what is
    # running" is most useful as something a person can connect with.
    if [ "$running" = yes ]; then
      printf '  %-18s %s\n' "$name" "$url"
      PASSED+=("$name")
    fi
    continue
    ;;
  dsn)
    # The dburl style URL, which is what a person pastes into usql. It differs
    # from the driver DSN for MySQL and Cassandra, whose drivers take a form
    # that is not a URL.
    printf '%-18s %s\n' "$name" "$url"
    PASSED+=("$name")
    continue
    ;;
  usql)
    if [ "$running" != yes ]; then
      continue
    fi
    if ! command -v usql >/dev/null 2>&1; then
      echo "usql is not on the path"
      exit 1
    fi
    echo "  connecting to ${name}"
    usql "$url"
    continue
    ;;
  version)
    # Silent for a server that is not up, the same as status: both answer
    # about what is running and a list of what is not is not an answer.
    if [ "$running" != yes ]; then
      continue
    fi
    # The dialect is the environment variable without its prefix, lower
    # cased, which is how tool/servers names both.
    dialect=$(printf '%s' "${envvar#DBMETA_}" | tr '[:upper:]' '[:lower:]')
    if reported=$(go run ./tool/version "$dialect" "$dsn" 2>&1); then
      printf '  %-18s %s\n' "$name" "$reported"
      PASSED+=("$name")
    else
      echo "  ${name}: $(printf '%s' "$reported" | tail -1)"
      FAILED+=("$name: no version")
    fi
    continue
    ;;
  stop)
    if [ "$running" = yes ]; then
      "$RUNNER" stop "$name" >/dev/null 2>&1
      echo "  stopped ${name}"
      PASSED+=("$name")
    fi
    continue
    ;;
  remove)
    "$RUNNER" "${REMOVE[@]}" >/dev/null 2>&1
    echo "  removed ${name}"
    PASSED+=("$name")
    continue
    ;;
  start)
    # A container that exists and is stopped is started rather than rebuilt,
    # so that a server keeps whatever was created in it.
    if [ "$running" = yes ]; then
      echo "  ${name} is already up: $envvar=$dsn"
      PASSED+=("$name")
      continue
    fi
    if "$RUNNER" container exists "$name" 2>/dev/null; then
      if "$RUNNER" start "$name" >/dev/null 2>&1; then
        echo "  ${name} is up: $envvar=$dsn"
        PASSED+=("$name")
        continue
      fi
      "$RUNNER" "${REMOVE[@]}" >/dev/null 2>&1
    fi
    ;;
  test)
    # remove a container left behind by a run that was interrupted
    "$RUNNER" "${REMOVE[@]}" >/dev/null 2>&1
    ;;
  esac

  # Cassandra is the one product whose image this repository builds. Build it
  # here rather than making every caller remember, which is what CI had to do
  # before and what a person reading the failure could not guess.
  case "$name" in
  cassandra-*)
    tag="localhost/dbmeta/cassandra:${name#cassandra-}"
    if ! "$RUNNER" image exists "$tag" 2>/dev/null; then
      echo "  building $tag"
      if ! "$HERE/cassandra/build.sh" "${name#cassandra-}" >/dev/null 2>&1; then
        echo "  could not build $tag"
        FAILED+=("$name: no image")
        continue
      fi
    fi
    ;;
  esac

  if ! err=$("$RUNNER" "${RUN[@]}" 2>&1); then
    echo "  could not start ${name}, skipping"
    echo "  ${RUNNER}: $(printf '%s' "$err" | tail -1)"
    # A run that was killed can leave the name held in container storage
    # where rm --force does not reach it. Say the one command that clears it,
    # because the message above says only that the name is in use.
    case "$err" in
    *"already in use"*)
      echo "  clear it with: $RUNNER rm --storage ${name}"
      ;;
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

  if [ "$MODE" = start ]; then
    echo "  ${name} is up: $envvar=$dsn"
    PASSED+=("$name")
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
if [ "$MODE" != test ]; then
  echo "${MODE}: ${PASSED[*]:-nothing}"
  if [ ${#FAILED[@]} -ne 0 ]; then
    echo "failed: ${FAILED[*]}"
    exit 1
  fi
  exit 0
fi
if [ ${#FAILED[@]} -eq 0 ]; then
  echo "every release passed: ${PASSED[*]}"
  exit 0
fi
echo "passed: ${PASSED[*]}"
echo "failed: ${FAILED[*]}"
exit 1
