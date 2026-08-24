# Forgejo origin helpers for muxcore-installer.
# shellcheck shell=bash

forgejo_base_url() {
  printf '%s\n' "${MUXCORE_FORGEJO_URL:-https://git.zem.systems}"
}

forgejo_org() {
  printf '%s\n' "${MUXCORE_FORGEJO_ORG:-muxcore}"
}

forgejo_token() {
  if [[ -n "${FORGEJO_TOKEN:-}" ]]; then
    printf '%s' "$FORGEJO_TOKEN"
    return 0
  fi
  if [[ -n "${MUXCORE_FORGEJO_TOKEN:-}" ]]; then
    printf '%s' "$MUXCORE_FORGEJO_TOKEN"
    return 0
  fi
  local f
  for f in \
    "${MUXCORE_FORGEJO_TOKEN_FILE:-}" \
    "$HOME/.config/muxcore/forgejo.token" \
    "$HOME/.config/forgejo/token"; do
    [[ -n "$f" && -f "$f" ]] || continue
    tr -d '[:space:]' <"$f"
    return 0
  done
  return 1
}

forgejo_release_download_url() {
  local repo="$1" tag="$2" asset="$3"
  printf '%s/%s/%s/releases/download/%s/%s' \
    "$(forgejo_base_url)" "$(forgejo_org)" "$repo" "$tag" "$asset"
}

forgejo_curl() {
  local token
  if token="$(forgejo_token 2>/dev/null || true)" && [[ -n "$token" ]]; then
    curl -fsSL -H "Authorization: token ${token}" "$@"
  else
    curl -fsSL "$@"
  fi
}
