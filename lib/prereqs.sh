# Dependency checks. Never run the wizard as root. Docker is opt-in.
# shellcheck shell=bash

PREREQS_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PREREQS_ROOT="${PREREQS_ROOT:-$(cd "$PREREQS_LIB_DIR/.." && pwd)}"

# shellcheck disable=SC1091
source "$PREREQS_LIB_DIR/common.sh"
# shellcheck disable=SC1091
source "$PREREQS_LIB_DIR/ui.sh"

GUM_VERSION="${GUM_VERSION:-v0.14.5}"
GUM_CACHE="${PREREQS_ROOT}/cache/gum"
GUM_BIN="${GUM_BIN:-}"
PREREQS_USED_SUDO=0

prereqs_refuse_root() {
  if [[ "$(id -u)" -eq 0 ]]; then
    ui_die "do not run this installer as root. Run it as your normal user (it will ask for sudo only when installing packages)."
  fi
}

prereqs_os_family() {
  if [[ "$(uname -s)" == "Darwin" ]]; then
    printf '%s\n' "darwin"
    return 0
  fi
  if [[ -f /etc/os-release ]]; then
    # shellcheck disable=SC1091
    source /etc/os-release
    case "${ID:-}" in
      debian|ubuntu|raspbian|linuxmint|pop) printf '%s\n' "debian"; return 0 ;;
      fedora|rhel|centos|rocky|almalinux) printf '%s\n' "rhel"; return 0 ;;
      arch|manjaro|endeavouros) printf '%s\n' "arch"; return 0 ;;
      alpine) printf '%s\n' "alpine"; return 0 ;;
      nixos) printf '%s\n' "nixos"; return 0 ;;
    esac
    case "${ID_LIKE:-}" in
      *debian*) printf '%s\n' "debian"; return 0 ;;
      *rhel*|*fedora*) printf '%s\n' "rhel"; return 0 ;;
    esac
  fi
  printf '%s\n' "unknown"
}

prereqs_sudo() {
  PREREQS_USED_SUDO=1
  sudo "$@"
}

prereqs_sudo_k() {
  if [[ "$PREREQS_USED_SUDO" -eq 1 ]] && command -v sudo >/dev/null 2>&1; then
    sudo -k 2>/dev/null || true
  fi
}

prereqs_install_packages() {
  local pkgs=("$@")
  local joined
  joined="$(IFS=' '; echo "${pkgs[*]}")"
  if ui_noninteractive; then
    ui_die "missing packages: $joined (install them, or re-run interactively)"
  fi
  echo
  echo "The installer needs: $joined"
  if ! ui_confirm "Install these packages now? (will ask for your password)" true; then
    ui_die "required tools missing: $joined"
  fi
  case "$(prereqs_os_family)" in
    debian)
      prereqs_sudo env DEBIAN_FRONTEND=noninteractive apt-get update -qq
      prereqs_sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y "${pkgs[@]}"
      ;;
    rhel)
      if command -v dnf >/dev/null 2>&1; then
        prereqs_sudo dnf install -y "${pkgs[@]}"
      else
        prereqs_sudo yum install -y "${pkgs[@]}"
      fi
      ;;
    arch)
      prereqs_sudo pacman -Sy --noconfirm "${pkgs[@]}"
      ;;
    alpine)
      prereqs_sudo apk add --no-cache "${pkgs[@]}"
      ;;
    darwin)
      command -v brew >/dev/null 2>&1 || ui_die "install Homebrew from https://brew.sh then re-run setup"
      brew install "${pkgs[@]}"
      ;;
    nixos)
      echo "NixOS: add the tools yourself, then press Enter."
      echo "  nix-shell -p ${joined}"
      onboard_require_tty
      read -r -p "Press Enter when they are on PATH…" _ || true
      ;;
    *)
      ui_die "cannot auto-install packages on this OS. Please install: $joined"
      ;;
  esac
}

prereqs_ensure_cmd() {
  local cmd="$1"
  local pkg="${2:-$cmd}"
  if command -v "$cmd" >/dev/null 2>&1; then
    return 0
  fi
  prereqs_install_packages "$pkg"
  command -v "$cmd" >/dev/null 2>&1 || ui_die "still missing $cmd after install attempt"
}

prereqs_gum_asset_name() {
  local os="$1" arch="$2" ver="${GUM_VERSION#v}"
  local gum_os gum_arch
  case "$os" in
    linux) gum_os=Linux ;;
    darwin) gum_os=Darwin ;;
    *) return 1 ;;
  esac
  case "$arch" in
    amd64) gum_arch=x86_64 ;;
    arm64) gum_arch=arm64 ;;
    *) return 1 ;;
  esac
  printf 'gum_%s_%s_%s.tar.gz' "$ver" "$gum_os" "$gum_arch"
}

prereqs_ensure_curl_first() {
  command -v curl >/dev/null 2>&1 && return 0
  echo "==> curl is required to continue"
  case "$(prereqs_os_family)" in
    debian)
      if command -v sudo >/dev/null 2>&1; then
        prereqs_sudo env DEBIAN_FRONTEND=noninteractive apt-get update -qq
        prereqs_sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y curl
      else
        ui_die "install curl manually, then re-run setup"
      fi
      ;;
    nixos)
      ui_die "install curl (nix-shell -p curl) then re-run setup"
      ;;
    *)
      ui_die "install curl manually, then re-run setup"
      ;;
  esac
}

prereqs_verify_gum_checksum() {
  local tarball="$1" sums="$2" name want got
  name="$(basename "$tarball")"
  [[ -s "$sums" ]] || return 0
  want="$(awk -v n="$name" '$2==n || $2=="*"n {print $1; exit}' "$sums" 2>/dev/null || true)"
  [[ -n "$want" ]] || return 0
  if command -v sha256sum >/dev/null 2>&1; then
    got="$(sha256sum "$tarball" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    got="$(shasum -a 256 "$tarball" | awk '{print $1}')"
  else
    return 0
  fi
  [[ "$got" == "$want" ]] || ui_die "gum checksum mismatch (got $got want $want)"
}

prereqs_ensure_gum() {
  if [[ -n "$GUM_BIN" && -x "$GUM_BIN" ]]; then
    export GUM_BIN
    return 0
  fi
  if command -v gum >/dev/null 2>&1; then
    GUM_BIN="$(command -v gum)"
    export GUM_BIN
    return 0
  fi
  local cached="${GUM_CACHE}/gum"
  if [[ -x "$cached" ]]; then
    GUM_BIN="$cached"
    export GUM_BIN
    return 0
  fi

  prereqs_ensure_curl_first
  local os arch asset url tmp sums
  read -r os arch < <(detect_os_arch)
  asset="$(prereqs_gum_asset_name "$os" "$arch")" || {
    ui_warn "no Gum build for ${os}/${arch} — using dialog/whiptail/prompts"
    return 0
  }
  url="https://github.com/charmbracelet/gum/releases/download/${GUM_VERSION}/${asset}"
  mkdir -p "$GUM_CACHE"
  tmp="$(mktemp -d "${GUM_CACHE}/dl.XXXXXX")"
  echo "==> downloading Gum ${GUM_VERSION}"
  if ! curl --proto '=https' --tlsv1.2 -fsSL -o "$tmp/$asset" "$url"; then
    ui_warn "could not download Gum — falling back to dialog/whiptail/prompts"
    rm -rf "$tmp"
    return 0
  fi
  sums="$tmp/checksums.txt"
  curl --proto '=https' --tlsv1.2 -fsSL -o "$sums" \
    "https://github.com/charmbracelet/gum/releases/download/${GUM_VERSION}/checksums.txt" \
    2>/dev/null || true
  prereqs_verify_gum_checksum "$tmp/$asset" "$sums"
  tar -xzf "$tmp/$asset" -C "$tmp"
  local bin
  bin="$(find "$tmp" -type f -name gum | head -1)"
  if [[ -z "$bin" || ! -s "$bin" ]]; then
    ui_warn "gum archive had no binary — falling back"
    rm -rf "$tmp"
    return 0
  fi
  install -m 0755 "$bin" "$cached"
  rm -rf "$tmp"
  GUM_BIN="$cached"
  export GUM_BIN
}

prereqs_ensure_ffmpeg() {
  if command -v ffmpeg >/dev/null 2>&1 && command -v ffprobe >/dev/null 2>&1; then
    ui_ok "ffmpeg / ffprobe present"
    return 0
  fi
  if ui_noninteractive; then
    ui_warn "ffmpeg/ffprobe missing — playback and file analysis will be limited"
    return 0
  fi
  if ! ui_confirm "Install ffmpeg (needed to analyze video and play in the MuxCore player)?" true; then
    ui_warn "skipping ffmpeg — you can install it later"
    return 0
  fi
  case "$(prereqs_os_family)" in
    debian) prereqs_install_packages ffmpeg ;;
    rhel) prereqs_install_packages ffmpeg ;;
    arch) prereqs_install_packages ffmpeg ;;
    alpine) prereqs_install_packages ffmpeg ;;
    darwin) prereqs_install_packages ffmpeg ;;
    nixos)
      echo "  nix-shell -p ffmpeg"
      ;;
    *)
      ui_warn "install ffmpeg from your package manager"
      ;;
  esac
}

prereqs_have_compose() {
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    return 0
  fi
  if command -v podman >/dev/null 2>&1 && podman compose version >/dev/null 2>&1; then
    return 0
  fi
  if command -v docker-compose >/dev/null 2>&1; then
    return 0
  fi
  if command -v podman-compose >/dev/null 2>&1; then
    return 0
  fi
  return 1
}

prereqs_offer_docker() {
  if prereqs_have_compose || { command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; }; then
    ui_ok "Docker / Compose is available"
    return 0
  fi
  if ui_noninteractive; then
    ui_warn "Docker not found (optional)"
    return 0
  fi
  if ! ui_confirm "Install Docker? (only needed for Compose or a local Postgres container)" false; then
    ui_ok "skipping Docker"
    return 0
  fi
  cat <<'EOF'

This installer will not run Docker's convenience script for you.

Linux: follow https://docs.docker.com/engine/install/ (or install podman + podman-compose)
macOS: https://docs.docker.com/desktop/install/mac-install/

EOF
  onboard_require_tty
  read -r -p "Press Enter after Docker/Podman is installed, or Ctrl-C to cancel…" _ || true
  if prereqs_have_compose || command -v docker >/dev/null 2>&1; then
    ui_ok "Docker / Compose detected"
  else
    ui_warn "Docker still not on PATH — you can switch to host processes in the next step"
  fi
}

prereqs_detect_systemd_user() {
  command -v systemctl >/dev/null 2>&1 && systemctl --user status >/dev/null 2>&1
}

prereqs_bootstrap() {
  prereqs_refuse_root
  ui_step 0 10 "Prepare your system"
  ui_info "Checking tools needed for setup…"

  prereqs_ensure_curl_first
  prereqs_ensure_cmd tar tar
  if ! command -v bash >/dev/null 2>&1; then
    ui_die "bash is required"
  fi

  echo "==> setting up the interactive UI"
  prereqs_ensure_gum
  UI_BACKEND=""
  ui_detect_backend
  ui_ok "UI backend: $UI_BACKEND"

  read -r _os _arch < <(detect_os_arch)
  ui_ok "platform ${_os}/${_arch}"
}
