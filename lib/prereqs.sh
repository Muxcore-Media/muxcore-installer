# Bootstrap curl, git, tar, gum (+ optional Docker) for onboarding.
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
      arch|manjaro) printf '%s\n' "arch"; return 0 ;;
      alpine) printf '%s\n' "alpine"; return 0 ;;
    esac
    case "${ID_LIKE:-}" in
      *debian*) printf '%s\n' "debian"; return 0 ;;
      *rhel*|*fedora*) printf '%s\n' "rhel"; return 0 ;;
    esac
  fi
  printf '%s\n' "unknown"
}

prereqs_have_sudo() {
  command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null
}

prereqs_install_packages() {
  local pkgs=("$@")
  local joined
  joined="$(IFS=' '; echo "${pkgs[*]}")"
  if ui_noninteractive; then
    ui_die "missing packages: $joined (install manually or run interactively)"
  fi
  if ! ui_confirm "Install missing tools with administrator access? ($joined)" false; then
    ui_die "required tools missing: $joined"
  fi
  case "$(prereqs_os_family)" in
    debian)
      sudo env DEBIAN_FRONTEND=noninteractive apt-get update -qq
      sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y "${pkgs[@]}"
      ;;
    rhel)
      if command -v dnf >/dev/null 2>&1; then
        sudo dnf install -y "${pkgs[@]}"
      else
        sudo yum install -y "${pkgs[@]}"
      fi
      ;;
    arch)
      sudo pacman -Sy --noconfirm "${pkgs[@]}"
      ;;
    alpine)
      sudo apk add --no-cache "${pkgs[@]}"
      ;;
    darwin)
      command -v brew >/dev/null 2>&1 || ui_die "install Homebrew from https://brew.sh then re-run setup"
      brew install "${pkgs[@]}"
      ;;
    *)
      ui_die "cannot auto-install packages on this OS. Please install: $joined"
      ;;
  esac
}

prereqs_ensure_cmd() {
  local cmd="$1"
  shift
  local apt_name="${1:-$cmd}"
  if command -v "$cmd" >/dev/null 2>&1; then
    return 0
  fi
  case "$(prereqs_os_family)" in
    debian) prereqs_install_packages "$apt_name" ;;
    rhel)
      case "$apt_name" in
        curl) prereqs_install_packages curl ;;
        git) prereqs_install_packages git ;;
        tar) prereqs_install_packages tar ;;
        *) prereqs_install_packages "$apt_name" ;;
      esac
      ;;
    arch|alpine|darwin) prereqs_install_packages "$apt_name" ;;
    *)
      ui_die "missing required command: $cmd"
      ;;
  esac
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
  if command -v curl >/dev/null 2>&1; then
    return 0
  fi
  echo "==> curl is required to continue"
  case "$(prereqs_os_family)" in
    debian)
      if command -v sudo >/dev/null 2>&1; then
        sudo env DEBIAN_FRONTEND=noninteractive apt-get update -qq
        sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y curl
      else
        ui_die "install curl manually, then re-run setup"
      fi
      ;;
    *)
      ui_die "install curl manually, then re-run setup"
      ;;
  esac
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
  local os arch asset url tmp
  read -r os arch < <(detect_os_arch)
  asset="$(prereqs_gum_asset_name "$os" "$arch")" || ui_die "unsupported platform for gum auto-download: ${os}/${arch}"
  url="https://github.com/charmbracelet/gum/releases/download/${GUM_VERSION}/${asset}"
  mkdir -p "$GUM_CACHE"
  tmp="$(mktemp -d "${GUM_CACHE}/dl.XXXXXX")"
  ui_spin "Downloading Gum ${GUM_VERSION}…" curl -fsSL -o "$tmp/$asset" "$url" \
    || { echo "==> downloading Gum ${GUM_VERSION}…"; curl -fsSL -o "$tmp/$asset" "$url"; }
  tar -xzf "$tmp/$asset" -C "$tmp"
  local bin
  bin="$(find "$tmp" -type f -name gum | head -1)"
  [[ -n "$bin" && -s "$bin" ]] || ui_die "gum download did not contain a binary"
  install -m 0755 "$bin" "$cached"
  rm -rf "$tmp"
  GUM_BIN="$cached"
  export GUM_BIN
}

prereqs_offer_docker() {
  if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    ui_ok "Docker is available (optional — for Postgres profile)"
    return 0
  fi
  if ! ui_confirm "Install Docker? (optional — only needed for MUXCORE_PROFILE=postgres)" false; then
    ui_ok "Skipping Docker — default install uses SQLite"
    return 0
  fi
  case "$(prereqs_os_family)" in
    debian|rhel|unknown)
      if [[ "$(prereqs_os_family)" == debian || "$(prereqs_os_family)" == rhel ]]; then
        :
      fi
      ui_spin "Installing Docker…" sh -c 'curl -fsSL https://get.docker.com | sudo sh'
      sudo usermod -aG docker "$USER" 2>/dev/null || true
      ui_ok "Docker installed — you may need to log out and back in for group membership"
      ;;
    darwin)
      ui_info "On macOS, install Docker Desktop: https://docs.docker.com/desktop/install/mac-install/"
      ;;
    *)
      ui_info "Install Docker from https://docs.docker.com/get-docker/"
      ;;
  esac
}

prereqs_ensure_terminfo() {
  ui_fix_term
  ui_term_works && return 0
  case "$(prereqs_os_family)" in
    debian)
      if command -v sudo >/dev/null 2>&1 && ui_confirm "Install ncurses terminfo packages (fixes clear and terminal UI)?" true; then
        sudo env DEBIAN_FRONTEND=noninteractive apt-get update -qq
        sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y ncurses-term ncurses-base
        ui_fix_term
      fi
      ;;
  esac
}

prereqs_bootstrap() {
  if ui_noninteractive; then
    require_cmd curl tar bash || ui_die "missing curl, tar, or bash"
    command -v git >/dev/null 2>&1 || ui_die "missing git"
    prereqs_ensure_gum
    return 0
  fi

  ui_step 0 7 "Prepare your system"
  ui_info "Checking tools needed for setup…"

  prereqs_ensure_terminfo

  ui_spin "Checking curl…" true
  prereqs_ensure_cmd curl curl

  ui_spin "Checking tar…" true
  prereqs_ensure_cmd tar tar

  ui_spin "Checking git…" true
  prereqs_ensure_cmd git git

  ui_spin "Setting up the interactive UI…" prereqs_ensure_gum
  ui_ok "Gum ready ($GUM_BIN)"

  prereqs_offer_docker

  read -r _os _arch < <(detect_os_arch)
  ui_ok "platform ${_os}/${_arch}"
}
