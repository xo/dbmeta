#!/bin/bash
# Provisions a Windows virtual machine that hosts an old SQL Server.
#
# SQL Server on Linux begins at 2017, so 2016 and earlier have no container and
# cannot run in CI. This is how those releases get tested at all. See D57.
#
# It reads the machine list from the Go package github.com/xo/dbmeta/container
# through ./tool/vms, so this script holds no copy of it. Add a release there.
#
# Usage:
#   ./vm/provision.sh 2012          provision one release
#   ./vm/provision.sh               provision every release, one at a time
#   ./vm/provision.sh 2012 --watch  print the web console URL and wait
#   ./vm/provision.sh 2012 --render write the OEM folder and stop
#
# It needs podman and /dev/kvm. Set DBMETA_RUNNER=docker to use docker.
#
# What it does, per release:
#   1. downloads the SQL Server Express installer on this host, because
#      Windows Server 2008 R2 has no TLS 1.2 and cannot fetch it itself
#   2. writes an OEM folder holding that installer, a configuration file and
#      install.bat with the per release values filled in
#   3. starts dockurr/windows with that folder mounted at /oem, which Windows
#      setup copies to C:\OEM and runs install.bat from at the end
#   4. waits for SQL Server to answer on the host port
#
# The Windows images are Microsoft evaluation editions, which are free for 180
# days of testing and need no activation and no product key. Nothing here
# activates Windows. When 180 days runs out, `slmgr /rearm` inside the machine
# extends it, which is Microsoft's own mechanism.
#
# The first run of a release takes 30 to 60 minutes: Windows installs, then SQL
# Server installs. Later runs start an existing machine in about a minute.

set -u

RUNNER="${DBMETA_RUNNER:-podman}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Where the machine disks live. Not in the repository: a Windows disk is tens
# of gigabytes and a grep or an editor index over the working tree should not
# have to walk it. The XDG data directory is the conventional home, and
# DBMETA_VM_STATE overrides it.
STATE="${DBMETA_VM_STATE:-${XDG_DATA_HOME:-$HOME/.local/share}/dbmeta/vm}"
SAPWD='P4ssw0rd!x'

if ! command -v "$RUNNER" >/dev/null 2>&1; then
  echo "$RUNNER is not on the path. Set DBMETA_RUNNER to the one you have."
  exit 1
fi
if [ ! -e /dev/kvm ]; then
  echo "/dev/kvm is missing. Without it QEMU emulates, and a Windows install"
  echo "that takes 30 minutes takes most of a day. Enable virtualization."
  exit 1
fi

WATCH=no
RENDER=no
ARGS=()
for arg in "$@"; do
  case "$arg" in
    --watch) WATCH=yes ;;
    # Write the OEM folder and stop, without downloading anything or starting
    # a machine. It is how the rendering is checked, because the alternative
    # is a forty minute install per attempt.
    --render) RENDER=yes ;;
    *) ARGS+=("$arg") ;;
  esac
done

LIST=$(cd "$HERE/.." && go run ./tool/vms "${ARGS[@]+"${ARGS[@]}"}") || exit 1
if [ -z "$LIST" ]; then
  echo "no machine matches: ${ARGS[*]-}"
  exit 1
fi

PASSED=()
FAILED=()

# The fields are separated by a unit separator rather than a tab, because the
# license field is empty on 2008 R2 and bash collapses a run of tabs. See
# ./tool/vms.
while IFS=$'\x1f' read -r name release image port viewer regkey license url file dsn; do
  printf '=== %s (SQL Server %s on %s) ===\n' "$name" "$release" "$image"
  oem="$STATE/$name/oem"
  shared="$STATE/$name/shared"
  mkdir -p "$oem" "$STATE/$name/storage" "$shared"
  # install.bat looks for this marker to find the shared drive, because the
  # letter Windows gives it varies by release.
  : > "$shared/.shared"

  # 1. the installer, fetched here rather than in the machine
  if [ "$RENDER" = yes ]; then
    echo "  --render given, not downloading $file"
  elif [ ! -s "$oem/$file" ]; then
    echo "  downloading $file"
    if ! curl -fSL --retry 3 -o "$oem/$file" "$url"; then
      echo "  could not download the installer"
      FAILED+=("$name: no installer")
      continue
    fi
  else
    echo "  installer already present"
  fi

  # 2. the OEM payload, with the per release values filled in
  cp "$HERE/oem/ConfigurationFile.ini" "$oem/ConfigurationFile.ini"
  if [ "$release" = "2008R2" ]; then
    # 2008 R2 wants its own section header. It does take the license flag,
    # despite a review saying otherwise, and refuses to install without it.
    sed -i 's/^\[OPTIONS\]$/[SQLSERVER2008]/' "$oem/ConfigurationFile.ini"
  fi
  sed -e "s|@@REGISTRY_KEY@@|$regkey|g" \
      -e "s|@@SA_PASSWORD@@|$SAPWD|g" \
      -e "s|@@INSTALLER_FILE@@|$file|g" \
      -e "s|@@LICENSE_FLAG@@|$license|g" \
      "$HERE/oem/install.bat" > "$oem/install.bat"
  # Windows reads a batch file, so the line endings have to be its own.
  sed -i 's/$/\r/' "$oem/install.bat" "$oem/ConfigurationFile.ini"

  if [ "$RENDER" = yes ]; then
    echo "  wrote $oem/install.bat and $oem/ConfigurationFile.ini"
    PASSED+=("$name")
    continue
  fi

  # 3. the machine
  if $RUNNER container exists "$name" 2>/dev/null; then
    echo "  the machine exists, starting it"
    $RUNNER start "$name" >/dev/null
  else
    echo "  creating the machine, which installs Windows and then SQL Server"
    $RUNNER run --detach --name "$name" \
      --env VERSION="$image" \
      --env DISK_SIZE="64G" \
      --env RAM_SIZE="4G" \
      --env CPU_CORES="4" \
      --publish "127.0.0.1:$port:1433" \
      --publish "127.0.0.1:$viewer:8006" \
      --device=/dev/kvm --device=/dev/net/tun --cap-add NET_ADMIN \
      --volume "$STATE/$name/storage:/storage" \
      --volume "$oem:/oem" \
      --volume "$shared:/shared" \
      --stop-timeout 120 \
      docker.io/dockurr/windows >/dev/null || {
        echo "  could not start the machine"
        FAILED+=("$name: will not start")
        continue
      }
  fi
  echo "  watch it at http://127.0.0.1:$viewer"

  if [ "$WATCH" = yes ]; then
    echo "  --watch given, leaving it running"
    PASSED+=("$name")
    continue
  fi

  # 4. wait for SQL Server to answer a query.
  #
  # Not a port check. The runtime publishes the port when the container is
  # created, so a connection to it succeeds within seconds and keeps
  # succeeding while Windows is still installing. The first version of this
  # reported every machine ready twenty seconds in. ./tool/vms -wait opens a
  # real connection and runs a statement.
  echo "  waiting for SQL Server on 127.0.0.1:$port"
  echo "  the first run installs Windows and then SQL Server, so allow an hour"
  if ! (cd "$HERE/.." && go run ./tool/vms -wait 90m "$release"); then
    echo "  it never answered. The install log, if it got that far:"
    echo "    $shared/provision-*.log"
    echo "  and the screen is at http://127.0.0.1:$viewer"
    ls -la "$shared" 2>/dev/null | tail -3
    FAILED+=("$name: never answered")
    continue
  fi
  echo "  answering"
  echo "  $dsn"
  PASSED+=("$name")
done <<< "$LIST"

echo
if [ ${#FAILED[@]} -eq 0 ]; then
  echo "provisioned: ${PASSED[*]}"
  exit 0
fi
echo "provisioned: ${PASSED[*]-none}"
echo "failed: ${FAILED[*]}"
exit 1
