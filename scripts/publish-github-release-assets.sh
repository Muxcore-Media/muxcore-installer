#!/usr/bin/env bash
# Publish linux/amd64 release tarballs to GitHub from a flat bin/ directory.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck disable=SC1091
source "$ROOT/versions.env"

GITHUB_ORG="${GITHUB_ORG:-Muxcore-Media}"
BIN_DIR="${1:-${MUXCORE_PUBLISH_BIN:-}}"
export GITHUB_TOKEN="${GITHUB_TOKEN:-${GH_TOKEN:-$(gh auth token 2>/dev/null || true)}}"

die() { echo "error: $*" >&2; exit 1; }
[[ -n "$GITHUB_TOKEN" ]] || die "set GITHUB_TOKEN or run: gh auth login"
[[ -n "$BIN_DIR" && -d "$BIN_DIR" ]] || die "usage: $0 /path/to/bin"

ensure_release() {
  local repo="$1" tag="$2"
  if gh release view "$tag" --repo "${GITHUB_ORG}/${repo}" >/dev/null 2>&1; then
    return 0
  fi
  gh release create "$tag" --repo "${GITHUB_ORG}/${repo}" --title "$tag" --notes "Published by muxcore-installer publish script"
}

publish_one() {
  local repo="$1" tag="$2" bin_name="$3"
  local ver="${tag#v}"
  local asset="${bin_name}_${ver}_linux_amd64.tar.gz"
  local src="$BIN_DIR/$bin_name"
  [[ -x "$src" ]] || { echo "  - skip $repo@$tag (missing $src)"; return 0; }
  echo "==> $repo@$tag"
  ensure_release "$repo" "$tag"
  local tmp
  tmp="$(mktemp -d)"
  cp "$src" "$tmp/$bin_name"
  tar -C "$tmp" -czf "$tmp/$asset" "$bin_name"
  gh release upload "$tag" --repo "${GITHUB_ORG}/${repo}" "$tmp/$asset" --clobber
  rm -rf "$tmp"
  echo "    uploaded $asset"
}

echo "==> publishing to github.com/${GITHUB_ORG} from $BIN_DIR"
publish_one core "$CORE_TAG" muxcored

while IFS= read -r line; do
  [[ -z "$line" || "$line" =~ ^# ]] && continue
  publish_one "${line%%=*}" "${line#*=}" "${line%%=*}"
done <<<"$MODULES"

auth_local_tag=""
while IFS= read -r line; do
  [[ "${line%%=*}" == "auth-local" ]] && auth_local_tag="${line#*=}" && break
done <<<"$MODULES"
[[ -n "$auth_local_tag" && -x "$BIN_DIR/authctl" ]] && publish_one auth-local "$auth_local_tag" authctl
[[ -x "$BIN_DIR/gettoken" ]] && publish_one muxcorectl-cli v0.1.0 gettoken

echo "done"
