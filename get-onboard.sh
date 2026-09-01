#!/usr/bin/env bash
# Landing script for: curl --proto '=https' --tlsv1.2 -fsSL https://getmuxcore.zem.systems | bash
# Downloads a pinned, checksummed muxcore-setup binary (a self-contained
# Bubbletea TUI — no bash wizard, no separate lib/ scripts to fetch) and execs
# it. No sudo. No package installs. No module downloads happen here.
set -euo pipefail

# Baked pin — override with MUXCORE_INSTALLER_TAG. Not "latest".
INSTALLER_TAG="${MUXCORE_INSTALLER_TAG:-v0.3.3}"
INSTALLER_REPO="${MUXCORE_INSTALLER_REPO:-muxcore-installer}"
FORGEJO_URL="${MUXCORE_FORGEJO_URL:-${FORGEJO_URL:-https://git.zem.systems}}"
FORGEJO_ORG="${MUXCORE_FORGEJO_ORG:-${FORGEJO_ORG:-muxcore}}"
GITHUB_ORG="${MUXCORE_GITHUB_ORG:-Muxcore-Media}"
BIN_NAME="muxcore-setup"

die() { echo "error: $*" >&2; exit 1; }

refuse_windows() {
  case "$(uname -s 2>/dev/null || echo unknown)" in
    MINGW*|MSYS*|CYGWIN*|Windows_NT|windows)
      cat >&2 <<'EOF'
error: native Windows is not supported.
Install MuxCore inside WSL2 (Ubuntu), then re-run:

  curl --proto '=https' --tlsv1.2 -fsSL https://getmuxcore.zem.systems | bash
EOF
      exit 1
      ;;
  esac
}

detect_os_arch() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) die "unsupported architecture: $arch (need amd64 or arm64)" ;;
  esac
  case "$os" in
    linux|darwin) ;;
    *) die "unsupported OS: $os (need Linux or macOS — use WSL2 on Windows)" ;;
  esac
  printf '%s %s\n' "$os" "$arch"
}

rebind_tty() {
  if [[ -t 0 ]]; then
    return 0
  fi
  if [[ -r /dev/tty ]] && exec </dev/tty 2>/dev/null; then
    return 0
  fi
  if [[ "${MUXCORE_NONINTERACTIVE:-}" == "1" ]]; then
    return 0
  fi
  cat >&2 <<'EOF'
error: this installer needs a terminal (stdin is not a TTY).

Safer two-step:

  curl --proto '=https' --tlsv1.2 -fsSL https://getmuxcore.zem.systems -o get-muxcore.sh
  bash get-muxcore.sh
EOF
  exit 1
}

bootstrap_forgejo_token() {
  if [[ -n "${FORGEJO_TOKEN:-}" ]]; then printf '%s' "$FORGEJO_TOKEN"; return 0; fi
  if [[ -n "${MUXCORE_FORGEJO_TOKEN:-}" ]]; then printf '%s' "$MUXCORE_FORGEJO_TOKEN"; return 0; fi
  local f
  for f in \
    "${MUXCORE_FORGEJO_TOKEN_FILE:-}" \
    "$HOME/.config/muxcore/forgejo.token"; do
    [[ -n "$f" && -f "$f" && -r "$f" ]] || continue
    tr -d '[:space:]' <"$f"
    return 0
  done
  return 1
}

bootstrap_github_token() {
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then printf '%s' "$GITHUB_TOKEN"; return 0; fi
  if [[ -n "${GH_TOKEN:-}" ]]; then printf '%s' "$GH_TOKEN"; return 0; fi
  if [[ -n "${MUXCORE_GITHUB_TOKEN:-}" ]]; then printf '%s' "$MUXCORE_GITHUB_TOKEN"; return 0; fi
  local f
  for f in \
    "${MUXCORE_GITHUB_TOKEN_FILE:-}" \
    "$HOME/.config/muxcore/github.token"; do
    [[ -n "$f" && -f "$f" && -r "$f" ]] || continue
    tr -d '[:space:]' <"$f"
    return 0
  done
  if command -v gh >/dev/null 2>&1; then
    gh auth token 2>/dev/null && return 0
  fi
  return 1
}

curl_auth() {
  local token="$1"; shift
  if [[ -n "$token" ]]; then
    curl --proto '=https' --tlsv1.2 -fsSL \
      -H "Authorization: Bearer ${token}" \
      -H "Accept: application/octet-stream" \
      "$@"
  else
    curl --proto '=https' --tlsv1.2 -fsSL "$@"
  fi
}

script_dir() {
  local here
  here="$(dirname "${BASH_SOURCE[0]:-$0}")"
  if here="$(cd "$here" 2>/dev/null && pwd)"; then
    printf '%s\n' "$here"
  else
    pwd
  fi
}

find_local_binary() {
  if [[ ! -t 0 && -z "${BASH_SOURCE[0]:-}" ]]; then
    return 1
  fi
  local here candidate
  here="$(script_dir)"
  for candidate in \
    "$here/$BIN_NAME" \
    "$here/bin/$BIN_NAME" \
    "$here/muxcore-installer/$BIN_NAME" \
    "$here/muxcore-installer/bin/$BIN_NAME" \
    "${PWD:-}/muxcore-installer/$BIN_NAME"; do
    if [[ -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  return 1
}

sha256_file() {
  local f="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$f" | awk '{print $1}'
    return 0
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$f" | awk '{print $1}'
    return 0
  fi
  return 1
}

verify_sha256() {
  local tarball="$1" sums="$2" want name got
  name="$(basename "$tarball")"
  want="$(awk -v n="$name" '$2==n || $2=="*"n {print $1; exit}' "$sums" 2>/dev/null || true)"
  [[ -n "$want" ]] || die "no SHA-256 entry for $name in SHA256SUMS"
  got="$(sha256_file "$tarball")" || die "no sha256sum/shasum on PATH to verify download"
  if [[ "$got" != "$want" ]]; then
    die "checksum mismatch for $name (got $got want $want)"
  fi
  echo "==> checksum ok ($name)" >&2
}

extract_binary() {
  local archive="$1" dest="$2"
  mkdir -p "$dest"
  tar -xzf "$archive" -C "$dest"
  if [[ -x "$dest/$BIN_NAME" ]]; then
    printf '%s\n' "$dest/$BIN_NAME"
    return 0
  fi
  local found
  found="$(find "$dest" -maxdepth 3 -type f -name "$BIN_NAME" 2>/dev/null | head -1 || true)"
  [[ -n "$found" ]] || return 1
  chmod u+x "$found"
  printf '%s\n' "$found"
}

forgejo_release_url() {
  local asset="$1"
  printf '%s/%s/%s/releases/download/%s/%s' "$FORGEJO_URL" "$FORGEJO_ORG" "$INSTALLER_REPO" "$INSTALLER_TAG" "$asset"
}

github_release_url() {
  local asset="$1"
  printf 'https://github.com/%s/%s/releases/download/%s/%s' "$GITHUB_ORG" "$INSTALLER_REPO" "$INSTALLER_TAG" "$asset"
}

asset_id_by_name_github() {
  local asset_name="$1" token="$2"
  command -v jq >/dev/null 2>&1 || return 1
  [[ -n "$token" ]] || return 1
  curl --proto '=https' --tlsv1.2 -fsSL \
    -H "Authorization: Bearer ${token}" \
    -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/${GITHUB_ORG}/${INSTALLER_REPO}/releases/tags/${INSTALLER_TAG}" \
    | jq -r --arg n "$asset_name" '.assets[]? | select(.name == $n) | .id' | head -1
}

fetch_release_asset() {
  local asset_name="$1" dest="$2"
  local fj_token gh_token id url
  fj_token="$(bootstrap_forgejo_token 2>/dev/null || true)"
  if curl_auth "$fj_token" -o "$dest" "$(forgejo_release_url "$asset_name")" 2>/dev/null; then
    return 0
  fi
  gh_token="$(bootstrap_github_token 2>/dev/null || true)"
  url="$(github_release_url "$asset_name")"
  if curl_auth "$gh_token" -o "$dest" "$url" 2>/dev/null; then
    return 0
  fi
  [[ -n "$gh_token" ]] || return 1
  id="$(asset_id_by_name_github "$asset_name" "$gh_token" 2>/dev/null || true)"
  [[ -n "$id" && "$id" != "null" ]] || return 1
  curl_auth "$gh_token" -o "$dest" "https://api.github.com/repos/${GITHUB_ORG}/${INSTALLER_REPO}/releases/assets/${id}"
}

fetch_binary() {
  command -v curl >/dev/null 2>&1 || die "curl is required to download the installer"
  command -v tar >/dev/null 2>&1 || die "tar is required to unpack the installer"

  local os arch ver cache dest tarball sums asset
  read -r os arch < <(detect_os_arch)
  ver="${INSTALLER_TAG#v}"
  cache="${XDG_CACHE_HOME:-$HOME/.cache}/muxcore-installer/${INSTALLER_TAG}"
  dest="$cache/${os}_${arch}"
  mkdir -p "$cache" "$dest"
  asset="${BIN_NAME}_${ver}_${os}_${arch}.tar.gz"
  tarball="$cache/${asset}"
  sums="$cache/SHA256SUMS"

  echo "==> fetching MuxCore installer ${INSTALLER_TAG} (${os}/${arch}) from Forgejo" >&2
  fetch_release_asset "$asset" "$tarball" \
    || die "could not download $BIN_NAME ${INSTALLER_TAG} for ${os}/${arch} (set FORGEJO_TOKEN or GITHUB_TOKEN if releases are private)"
  fetch_release_asset "SHA256SUMS" "$sums" \
    || die "could not download SHA256SUMS for ${INSTALLER_TAG} (required)"
  [[ -s "$sums" ]] || die "SHA256SUMS for ${INSTALLER_TAG} is missing or empty"
  verify_sha256 "$tarball" "$sums"
  extract_binary "$tarball" "$dest" || die "release tarball did not contain $BIN_NAME"
}

main() {
  command -v bash >/dev/null 2>&1 || die "bash is required"
  refuse_windows
  detect_os_arch >/dev/null
  rebind_tty

  local bin
  if bin="$(find_local_binary)"; then
    echo "==> using local build at $bin" >&2
  else
    bin="$(fetch_binary)"
  fi

  exec "$bin" "$@"
}

main "$@"
