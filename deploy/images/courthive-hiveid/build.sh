#!/bin/sh
# Build the courthive-hiveid bundle image locally. Nothing is pushed.
#
#   ./build.sh                 build the pinned commit as 0.5.0-jdt.1
#   ./build.sh <n>             same commit, image revision <n> (0.5.0-jdt.<n>)
#
# To move to a newer upstream commit, change UPSTREAM_COMMIT and
# UPSTREAM_VERSION below together (see README.md). The Dockerfile refuses to
# build when the fetched commit or its package.json version disagree with them.
set -eu
cd "$(dirname "$0")"

UPSTREAM_REPO=https://github.com/CourtHive/courthive-hiveid.git
UPSTREAM_COMMIT=7931efa18ea9db3751859d73ef771e9cd970748c
UPSTREAM_VERSION=0.5.0
IMAGE=ghcr.io/jameshartt/courthive-hiveid

n="${1:-1}"
case "$n" in ''|*[!0-9]*) echo "usage: ./build.sh [<n>]   (image tag $UPSTREAM_VERSION-jdt.<n>)" >&2; exit 2 ;; esac
version="$UPSTREAM_VERSION-jdt.$n"

docker build \
  --build-arg UPSTREAM_REPO="$UPSTREAM_REPO" \
  --build-arg UPSTREAM_COMMIT="$UPSTREAM_COMMIT" \
  --build-arg VERSION="$version" \
  -t "$IMAGE:$version" .
echo "built $IMAGE:$version"
