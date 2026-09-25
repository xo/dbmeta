#!/bin/bash
# Builds the Oracle 19c image, which Oracle publishes no free copy of.
#
# Every other Oracle release dbmeta tests against is a public image. 19c is the
# long term release most installations run and there is no free one, so it is
# built here from Oracle's own Dockerfiles and the installer archive, which a
# person downloads once under the developer licence.
#
# Usage:
#   ./oracle/build-19c.sh [path to LINUX.X64_193000_db_home.zip]
#
# With no argument it looks in ~/Downloads. The archive is about 3 GB and the
# build needs roughly 20 GB free and half an hour.
#
# It produces localhost/oracle/database:19.3.0-ee, which is what
# container/oracle.go names. Nothing else here downloads anything from Oracle,
# and this script does not either: the archive is the caller's to obtain and
# the licence is the caller's to accept.
#
# The archive is checked against the digest Oracle publishes before it is used.
# It arrives out of band, by hand, over a browser session, and it is three
# gigabytes that the build then runs as root inside an image, so it is worth
# knowing it is the file Oracle shipped and that it arrived whole.

set -u

RUNNER="${DBMETA_RUNNER:-podman}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Where the checkout and the build log live. Not in the repository, for the
# same reason as the machine disks: Oracle's Dockerfiles are a few hundred
# files that are not ours. DBMETA_ORACLE_STATE overrides it.
STATE="${DBMETA_ORACLE_STATE:-${XDG_DATA_HOME:-$HOME/.local/share}/dbmeta/oracle}"
ARCHIVE="${1:-$HOME/Downloads/LINUX.X64_193000_db_home.zip}"
IMAGE="localhost/oracle/database:19.3.0-ee"
REPO="https://github.com/oracle/docker-images.git"
# The digest Oracle publishes for LINUX.X64_193000_db_home.zip.
ARCHIVE_SHA256="ba8329c757133da313ed3b6d7f86c5ac42cd9970a28bf2e6233f3235233aa8d8"

if ! command -v "$RUNNER" >/dev/null 2>&1; then
  echo "$RUNNER is not on the path. Set DBMETA_RUNNER to the one you have."
  exit 1
fi
if [ ! -s "$ARCHIVE" ]; then
  echo "The installer archive is not at $ARCHIVE."
  echo "Download LINUX.X64_193000_db_home.zip from Oracle and pass its path."
  exit 1
fi
echo "checking the archive against the digest Oracle publishes"
got=$(sha256sum "$ARCHIVE" | cut -d" " -f1)
if [ "$got" != "$ARCHIVE_SHA256" ]; then
  echo "  the archive does not match."
  echo "    expected $ARCHIVE_SHA256"
  echo "    got      $got"
  echo "  Do not build from it. Download it again."
  exit 1
fi
echo "  matches"

if $RUNNER image exists "$IMAGE" 2>/dev/null; then
  echo "$IMAGE already exists. Remove it to build again."
  exit 0
fi

mkdir -p "$STATE"
tree="$STATE/docker-images"
if [ ! -d "$tree" ]; then
  # Only the one directory is needed, and the whole repository is large, so
  # this takes a shallow sparse checkout rather than a clone.
  echo "fetching Oracle's Dockerfiles"
  git clone --depth 1 --filter=blob:none --sparse "$REPO" "$tree" || exit 1
  (cd "$tree" && git sparse-checkout set OracleDatabase/SingleInstance) || exit 1
fi

dir="$tree/OracleDatabase/SingleInstance/dockerfiles"
if [ ! -d "$dir/19.3.0" ]; then
  echo "$dir/19.3.0 is missing. Oracle's layout has changed."
  exit 1
fi

# The build context must hold the archive. Link it rather than copy it, so a
# 3 GB file is not duplicated, and fall back to copying across filesystems.
if [ ! -s "$dir/19.3.0/$(basename "$ARCHIVE")" ]; then
  echo "placing the archive in the build context"
  ln "$ARCHIVE" "$dir/19.3.0/" 2>/dev/null || cp "$ARCHIVE" "$dir/19.3.0/" || exit 1
fi

# Oracle Linux 8, not 9.
#
# 19.3.0 is certified on Oracle Linux 7 and 8. Oracle's Dockerfile defaults to
# 9 and their build script overrides it to 8 only on ARM64, so an x86_64 build
# gets 9 and the relink fails:
#
#	[FATAL] Error in invoking target 'libasmclntsh19.ohso libasmperl19.ohso
#	client_sharedlib' of makefile ins_rdbms.mk
#
# and their script then prints "Build completed" and tags a 6.5 GB image that
# cannot open a database. That is why the build output is checked below rather
# than the exit status.
# One package has to go with the base change. setupLinuxEnv.sh installs
# libxcrypt-compat, which exists on Oracle Linux 9 to provide the older
# libcrypt.so.1. Oracle Linux 8 has that already and has no such package, so
# yum fails, the base layer never creates the oracle user, and the build dies
# later with "unknown user oracle". Everything else in that line installs on 8.
setup="$dir/19.3.0/setupLinuxEnv.sh"
if grep -q "libxcrypt-compat" "$setup"; then
  echo "dropping libxcrypt-compat, which Oracle Linux 8 neither has nor needs"
  sed -i 's/ libxcrypt-compat//' "$setup"
fi

echo "building $IMAGE on Oracle Linux 8, which takes about half an hour"
out="$STATE/build.log"
( cd "$dir" && CONTAINER_RUNTIME="$RUNNER" \
    ./buildContainerImage.sh -v 19.3.0 -e -o '--build-arg BASE_IMAGE=oraclelinux:8' ) \
  2>&1 | tee "$out"

# Oracle's script exits zero after a failed relink, so look at what it did.
if grep -q "\[FATAL\]" "$out"; then
  echo
  echo "the install failed, whatever the exit status said:"
  grep "\[FATAL\]" "$out" | head -3
  echo "the full output is in $out"
  $RUNNER rmi -f oracle/database:19.3.0-ee "$IMAGE" >/dev/null 2>&1
  exit 1
fi

# Oracle's script tags it oracle/database:19.3.0-ee. Name it the way
# container/oracle.go expects, if the two differ.
if ! $RUNNER image exists "$IMAGE" 2>/dev/null; then
  $RUNNER tag oracle/database:19.3.0-ee "$IMAGE" || exit 1
fi
echo "built $IMAGE"
