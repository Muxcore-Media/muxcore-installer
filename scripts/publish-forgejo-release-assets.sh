#!/usr/bin/env bash
# Publish linux/amd64 release tarballs to Forgejo from a flat bin/ directory.
# Run on vault (local Forgejo) or anywhere with FORGEJO_TOKEN + network to git.zem.systems.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck disable=SC1091
source "$ROOT/versions.env"

FORGEJO_URL="${FORGEJO_URL:-${MUXCORE_FORGEJO_URL:-https://git.zem.systems}}"
FORGEJO_ORG="${FORGEJO_ORG:-${MUXCORE_FORGEJO_ORG:-muxcore}}"
BIN_DIR="${1:-${MUXCORE_PUBLISH_BIN:-}}"
TOKEN="${FORGEJO_TOKEN:-${MUXCORE_FORGEJO_TOKEN:-}}"

die() { echo "error: $*" >&2; exit 1; }

json_field() {
  local json="$1" field="$2"
  if command -v jq >/dev/null 2>&1; then
    jq -r ".${field}" <<<"$json"
    return 0
  fi
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$field" <<<"$json"
}

[[ -n "$TOKEN" ]] || die "set FORGEJO_TOKEN (site admin or scoped releases+read on muxcore/*)"

api() {
  curl -fsS -H "Authorization: token ${TOKEN}" -H "Content-Type: application/json" "$@"
}

ensure_release() {
  local repo="$1" tag="$2"
  local existing
  existing="$(api "${FORGEJO_URL}/api/v1/repos/${FORGEJO_ORG}/${repo}/releases/tags/${tag}" 2>/dev/null || true)"
  if [[ -n "$existing" ]]; then
    json_field "$existing" id
    return 0
  fi
  local created
  created="$(api -X POST -d "{\"tag_name\":\"${tag}\",\"target_commitish\":\"main\",\"name\":\"${tag}\"}" \
    "${FORGEJO_URL}/api/v1/repos/${FORGEJO_ORG}/${repo}/releases")"
  json_field "$created" id
}

upload_asset() {
  local repo="$1" release_id="$2" asset_name="$3" file="$4"
  local assets existing
  assets="$(api "${FORGEJO_URL}/api/v1/repos/${FORGEJO_ORG}/${repo}/releases/${release_id}" 2>/dev/null || true)"
  if [[ -n "$assets" ]]; then
    if command -v jq >/dev/null 2>&1; then
      existing="$(jq -r --arg n "$asset_name" '.assets[]? | select(.name==$n) | .name' <<<"$assets" | head -1)"
    else
      existing="$(grep -F "\"name\":\"${asset_name}\"" <<<"$assets" && echo "$asset_name" || true)"
    fi
    if [[ "$existing" == "$asset_name" ]]; then
      echo "    skip existing asset $asset_name"
      return 0
    fi
  fi
  curl -fsS -X POST -H "Authorization: token ${TOKEN}" \
    -F "attachment=@${file}" \
    "${FORGEJO_URL}/api/v1/repos/${FORGEJO_ORG}/${repo}/releases/${release_id}/assets" >/dev/null
  echo "    uploaded $asset_name"
}

publish_one() {
  local repo="$1" tag="$2" bin_name="$3"
  local ver="${tag#v}"
  local asset="${bin_name}_${ver}_linux_amd64.tar.gz"
  local src="$BIN_DIR/$bin_name"
  [[ -x "$src" ]] || { echo "  - skip $repo@$tag (missing $src)"; return 0; }
  echo "==> $repo@$tag"
  local tmp release_id
  tmp="$(mktemp -d)"
  cp "$src" "$tmp/$bin_name"
  tar -C "$tmp" -czf "$tmp/$asset" "$bin_name"
  release_id="$(ensure_release "$repo" "$tag")"
  upload_asset "$repo" "$release_id" "$asset" "$tmp/$asset"
  rm -rf "$tmp"
}

[[ -n "$BIN_DIR" && -d "$BIN_DIR" ]] || die "usage: $0 /path/to/bin"

echo "==> publishing linux_amd64 assets from $BIN_DIR to ${FORGEJO_URL}/${FORGEJO_ORG}"

publish_one core "$CORE_TAG" muxcored

while IFS= read -r line; do
  [[ -z "$line" || "$line" =~ ^# ]] && continue
  repo="${line%%=*}"
  tag="${line#*=}"
  publish_one "$repo" "$tag" "$repo"
done <<<"$MODULES"

auth_local_tag=""
while IFS= read -r line; do
  [[ -z "$line" || "$line" =~ ^# ]] && continue
  [[ "${line%%=*}" == "auth-local" ]] && auth_local_tag="${line#*=}" && break
done <<<"$MODULES"
if [[ -n "$auth_local_tag" && -x "$BIN_DIR/authctl" ]]; then
  publish_one auth-local "$auth_local_tag" authctl
fi

if [[ -x "$BIN_DIR/gettoken" ]]; then
  publish_one muxcorectl-cli v0.1.0 gettoken 2>/dev/null || true
fi

echo "done"
