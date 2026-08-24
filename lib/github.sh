# GitHub release + API helpers for muxcore-installer.
# shellcheck shell=bash

github_org() {
  printf '%s\n' "${MUXCORE_GITHUB_ORG:-Muxcore-Media}"
}

github_token() {
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    printf '%s' "$GITHUB_TOKEN"
    return 0
  fi
  if [[ -n "${GH_TOKEN:-}" ]]; then
    printf '%s' "$GH_TOKEN"
    return 0
  fi
  if [[ -n "${MUXCORE_GITHUB_TOKEN:-}" ]]; then
    printf '%s' "$MUXCORE_GITHUB_TOKEN"
    return 0
  fi
  local f
  for f in \
    "${MUXCORE_GITHUB_TOKEN_FILE:-}" \
    "$HOME/.config/muxcore/github.token" \
    "$HOME/.config/gh/hosts.yml"; do
    [[ -n "$f" && -f "$f" ]] || continue
    if [[ "$f" == *hosts.yml ]]; then
      local tok
      tok="$(awk '/oauth_token:/ {print $2; exit}' "$f" 2>/dev/null || true)"
      [[ -n "$tok" ]] && printf '%s' "$tok" && return 0
      continue
    fi
    tr -d '[:space:]' <"$f"
    return 0
  done
  if command -v gh >/dev/null 2>&1; then
    gh auth token 2>/dev/null && return 0
  fi
  return 1
}

github_release_download_url() {
  local repo="$1" tag="$2" asset="$3"
  printf 'https://github.com/%s/%s/releases/download/%s/%s' \
    "$(github_org)" "$repo" "$tag" "$asset"
}

github_curl() {
  local token
  if token="$(github_token 2>/dev/null || true)" && [[ -n "$token" ]]; then
    curl -fsSL -H "Authorization: Bearer ${token}" -H "Accept: application/octet-stream" "$@"
  else
    curl -fsSL "$@"
  fi
}

github_api_curl() {
  local token
  if token="$(github_token 2>/dev/null || true)" && [[ -n "$token" ]]; then
    curl -fsSL -H "Authorization: Bearer ${token}" -H "Accept: application/vnd.github+json" "$@"
  else
    curl -fsSL -H "Accept: application/vnd.github+json" "$@"
  fi
}

github_download_release_asset() {
  local repo="$1" tag="$2" asset_name="$3" dest="$4"
  local token asset_id
  token="$(github_token 2>/dev/null || true)"
  [[ -n "$token" ]] || return 1
  asset_id="$(github_api_curl \
    "https://api.github.com/repos/$(github_org)/${repo}/releases/tags/${tag}" \
    | jq -r --arg n "$asset_name" '.assets[]? | select(.name == $n) | .id' | head -1)"
  [[ -n "$asset_id" && "$asset_id" != null ]] || return 1
  curl -fsSL \
    -H "Authorization: Bearer ${token}" \
    -H "Accept: application/octet-stream" \
    -o "$dest" \
    "https://api.github.com/repos/$(github_org)/${repo}/releases/assets/${asset_id}"
}
