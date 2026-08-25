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
    if [[ ! -r "$f" ]]; then
      echo "warning: cannot read GitHub token file $f (permission denied)" >&2
      continue
    fi
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

github_prompt_token() {
  local tok=""
  if [[ "${MUXCORE_NONINTERACTIVE:-}" == 1 ]]; then
    echo "error: GitHub returned a private/unavailable release and no token is set." >&2
    echo "Set GITHUB_TOKEN or write one to ~/.config/muxcore/github.token" >&2
    return 1
  fi
  echo
  echo "This GitHub release looks private (closed alpha). Paste a token with repo"
  echo "(and read:packages if you chose Docker Compose)."
  if type ui_password >/dev/null 2>&1; then
    ui_password tok "GitHub personal access token" ""
  else
    if [[ -r /dev/tty ]]; then
      printf 'GitHub token: ' >/dev/tty
      # shellcheck disable=SC2162
      read -s tok </dev/tty || true
      echo >/dev/tty
    else
      printf 'GitHub token: ' >&2
      # shellcheck disable=SC2162
      read -s tok || true
      echo >&2
    fi
  fi
  [[ -n "$tok" ]] || return 1
  mkdir -p "$HOME/.config/muxcore"
  printf '%s\n' "$tok" >"$HOME/.config/muxcore/github.token"
  chmod 600 "$HOME/.config/muxcore/github.token"
  export GITHUB_TOKEN="$tok"
  echo "saved token to ~/.config/muxcore/github.token"
  return 0
}

# Download a URL. Use a token when one is already set; otherwise try public.
# On 401/403/404 with no token, prompt and retry.
github_fetch() {
  local dest="$1"
  shift
  local code tmp token=""
  tmp="${dest}.partial"
  token="$(github_token 2>/dev/null || true)"
  if [[ -n "$token" ]]; then
    code="$(curl --proto '=https' --tlsv1.2 -sS -o "$tmp" -w '%{http_code}' \
      -H "Authorization: Bearer ${token}" \
      -H "Accept: application/octet-stream" \
      "$@" || echo 000)"
  else
    code="$(curl --proto '=https' --tlsv1.2 -sS -o "$tmp" -w '%{http_code}' "$@" || echo 000)"
  fi
  if [[ "$code" == 200 && -s "$tmp" ]]; then
    mv "$tmp" "$dest"
    return 0
  fi
  rm -f "$tmp"
  if [[ -z "$token" && ( "$code" == 401 || "$code" == 403 || "$code" == 404 ) ]]; then
    github_prompt_token || return 1
    token="$(github_token 2>/dev/null || true)"
    [[ -n "$token" ]] || return 1
    code="$(curl --proto '=https' --tlsv1.2 -sS -o "$tmp" -w '%{http_code}' \
      -H "Authorization: Bearer ${token}" \
      -H "Accept: application/octet-stream" \
      "$@" || echo 000)"
    if [[ "$code" == 200 && -s "$tmp" ]]; then
      mv "$tmp" "$dest"
      return 0
    fi
    rm -f "$tmp"
  fi
  return 1
}

github_download_release_asset() {
  local repo="$1" tag="$2" asset_name="$3" dest="$4"
  local url token asset_id
  url="$(github_release_download_url "$repo" "$tag" "$asset_name")"
  if github_fetch "$dest" "$url"; then
    return 0
  fi
  token="$(github_token 2>/dev/null || true)"
  [[ -n "$token" ]] || return 1
  command -v jq >/dev/null 2>&1 || return 1
  asset_id="$(github_api_curl \
    "https://api.github.com/repos/$(github_org)/${repo}/releases/tags/${tag}" \
    | jq -r --arg n "$asset_name" '.assets[]? | select(.name == $n) | .id' | head -1)"
  [[ -n "$asset_id" && "$asset_id" != null ]] || return 1
  github_fetch "$dest" \
    "https://api.github.com/repos/$(github_org)/${repo}/releases/assets/${asset_id}"
}
