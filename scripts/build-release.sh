#!/usr/bin/env bash
# Cross-compile muxcore-setup for every supported OS/arch and package each as
# the tarball get-onboard.sh expects: muxcore-setup_<ver>_<os>_<arch>.tar.gz,
# plus a combined SHA256SUMS. Run from the muxcore-installer checkout root.
#
# Usage:
#   ./scripts/build-release.sh [version]        # writes to ./dist/
#   VERSION=v0.3.1 ./scripts/build-release.sh    # same, via env
#
# Publish afterwards with gh, e.g.:
#   gh release create "$VERSION" ./dist/* --repo Muxcore-Media/muxcore-installer
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${1:-${VERSION:-}}"
if [[ -z "$VERSION" ]]; then
  # shellcheck disable=SC1091
  source "$ROOT/versions.env"
  VERSION="$INSTALLER_TAG"
fi
VER="${VERSION#v}"
DIST="$ROOT/dist"
BIN_NAME="muxcore-setup"

rm -rf "$DIST"
mkdir -p "$DIST"

targets=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
)

for t in "${targets[@]}"; do
  read -r os arch <<<"$t"
  echo "==> building ${os}/${arch}"
  tmp="$(mktemp -d)"
  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 \
    go build -ldflags="-s -w -X main.version=${VERSION}" \
    -o "$tmp/$BIN_NAME" ./cmd/muxcore-setup
  asset="${BIN_NAME}_${VER}_${os}_${arch}.tar.gz"
  tar -C "$tmp" -czf "$DIST/$asset" "$BIN_NAME"
  rm -rf "$tmp"
  echo "    -> dist/$asset"
done

(
  cd "$DIST"
  # get-onboard.sh matches $2==<bare filename> — no "./" prefix.
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum -- *.tar.gz >SHA256SUMS
  else
    shasum -a 256 -- *.tar.gz >SHA256SUMS
  fi
)

echo "==> done — artifacts in $DIST (version $VERSION)"
