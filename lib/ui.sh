# Glamorous onboarding UI (Charm Gum). Sourced by onboard.sh.
# shellcheck shell=bash

UI_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
UI_ROOT="${UI_ROOT:-$(cd "$UI_LIB_DIR/.." && pwd)}"
UI_BANNER="${UI_ROOT}/assets/banner.ascii"

# shellcheck disable=SC1091
[[ -f "${UI_ROOT}/versions.env" ]] && source "${UI_ROOT}/versions.env"

ui_noninteractive() {
  [[ "${MUXCORE_NONINTERACTIVE:-}" == 1 ]]
}

ui_term_works() {
  command -v tput >/dev/null 2>&1 && tput cols >/dev/null 2>&1
}

# SSH from Kitty/macOS often sets TERM=xterm-kitty; minimal containers lack that entry.
ui_fix_term() {
  [[ "${MUXCORE_TERM_FIXED:-}" == 1 ]] && return 0
  local orig="${TERM:-}" candidates=() t
  [[ -n "${MUXCORE_TERM:-}" ]] && candidates+=("$MUXCORE_TERM")
  [[ -n "$orig" ]] && candidates+=("$orig")
  candidates+=(xterm-256color xterm screen linux dumb)
  for t in "${candidates[@]}"; do
    TERM="$t"
    export TERM
    if ui_term_works; then
      MUXCORE_TERM_FIXED=1
      export MUXCORE_TERM_FIXED
      if [[ -n "$orig" && "$t" != "$orig" ]]; then
        echo "note: using TERM=$t ($orig is not available on this host)" >&2
      fi
      return 0
    fi
  done
  export TERM=dumb
  MUXCORE_TERM_FIXED=1
  export MUXCORE_TERM_FIXED
}

ui_gum() {
  if [[ -n "${GUM_BIN:-}" && -x "$GUM_BIN" ]]; then
    "$GUM_BIN" "$@"
    return $?
  fi
  if command -v gum >/dev/null 2>&1; then
    gum "$@"
    return $?
  fi
  return 127
}

# Run gum only when available; never call invalid subcommands (e.g. "gum true").
ui_gum_available() {
  ui_fix_term
  [[ "${TERM:-}" == dumb ]] && return 1
  ui_gum --version &>/dev/null
}

ui_gum_style() {
  ui_gum_available || return 1
  ui_gum style "$@"
}

ui_clear() {
  ui_fix_term
  if ui_term_works; then
    tput clear 2>/dev/null && return 0
  fi
  clear 2>/dev/null || printf '\033[2J\033[H' || true
}

ui_splash() {
  ui_clear
  if ui_gum_style --foreground 212 --bold "$(cat "$UI_BANNER")" 2>/dev/null; then
    :
  else
    cat "$UI_BANNER"
  fi
  echo
  ui_gum_style --foreground 240 "Personal media library setup for Linux and macOS" 2>/dev/null \
    || echo "Personal media library setup for Linux and macOS"
  echo
}

ui_step() {
  local n="$1" total="$2" title="$3"
  echo
  ui_gum_style --bold "Step ${n} of ${total} — ${title}" 2>/dev/null \
    || printf '\n── Step %s/%s — %s ──\n' "$n" "$total" "$title"
}

ui_info() {
  ui_gum_style --foreground 36 "$*" 2>/dev/null || echo "==> $*"
}

ui_ok() {
  ui_gum_style --foreground 42 "✓ $*" 2>/dev/null || echo "    $*"
}

ui_warn() {
  ui_gum_style --foreground 214 "⚠ $*" 2>/dev/null || echo "WARN: $*" >&2
}

ui_die() {
  ui_gum_style --foreground 196 --bold "error: $*" 2>/dev/null || echo "error: $*" >&2
  exit 1
}

ui_confirm() {
  local q="$1" def="${2:-false}"
  if ui_noninteractive; then
    [[ "$def" == true || "$def" == y || "$def" == yes ]]
    return
  fi
  if ui_gum_available; then
    local flag=()
    if [[ "$def" == true || "$def" == y || "$def" == yes ]]; then
      flag=(--default=true)
    else
      flag=(--default=false)
    fi
    ui_gum confirm "${flag[@]}" "$q"
    return
  fi
  onboard_require_tty
  local hint ans
  if [[ "$def" == true || "$def" == y || "$def" == yes ]]; then
    hint="Y/n"
  else
    hint="y/N"
  fi
  read -r -u "$_ONBOARD_READ_FD" -p "$q [$hint]: " ans || true
  ans="${ans:-$def}"
  [[ "$ans" =~ ^[Yy] ]]
}

ui_legal_gate() {
  if ui_noninteractive; then
    return 0
  fi
  local text
  text="$(cat <<'EOF'
MuxCore helps you organize and manage media you already have the legal right to use.

It does not provide, search for, or download copyrighted content. You are responsible
for complying with copyright and applicable laws in your region.

By continuing, you agree to use MuxCore only with lawfully obtained media.
EOF
)"
  ui_clear
  if ui_gum_available && ui_gum_style --bold "$(cat "$UI_BANNER")" 2>/dev/null; then
    echo
  else
    cat "$UI_BANNER"
    echo
  fi
  if ui_gum_available; then
    ui_gum_style --border double --padding "1 2" --width 72 "$text" 2>/dev/null || printf '%s\n' "$text"
    echo
    if ui_gum confirm --default=false --affirmative "I agree" --negative "Exit setup" \
      "I agree — I will only use MuxCore with media I have the right to use"; then
      return 0
    fi
  else
    printf '%s\n\n' "$text"
    if ui_confirm "I agree — I will only use MuxCore with media I have the right to use" false; then
      return 0
    fi
  fi
  echo "Setup cancelled. No changes were made."
  exit 0
}

ui_choose() {
  local var="$1"
  shift
  local choice default="${1:-}"
  if ui_noninteractive; then
    choice="$default"
  elif ui_gum_available; then
    choice="$(ui_gum choose "$@")" || return 1
  else
    local i=1 opt
    echo "$1" >&2
    for opt in "$@"; do
      [[ "$opt" == --* ]] && continue
      echo "  $i) $opt" >&2
      ((i++)) || true
    done
    onboard_require_tty
    read -r -u "$_ONBOARD_READ_FD" -p "Choose [1]: " choice || true
    choice="${choice:-1}"
    i=1
    for opt in "$@"; do
      [[ "$opt" == --* ]] && continue
      if [[ "$choice" == "$i" || "$choice" == "$opt" ]]; then
        choice="$opt"
        break
      fi
      ((i++)) || true
    done
  fi
  printf -v "$var" '%s' "$choice"
}

ui_input() {
  local var="$1" prompt="$2" def="${3:-}"
  shift 3 || true
  local val
  if ui_noninteractive; then
    val="$def"
  elif ui_gum_available; then
    if [[ -n "$def" ]]; then
      val="$(ui_gum input --placeholder "$def" --value "$def" --prompt "$prompt")" || return 1
    else
      val="$(ui_gum input --prompt "$prompt")" || return 1
    fi
    val="${val:-$def}"
  else
    onboard_prompt "$var" "$prompt" "$def"
    return 0
  fi
  printf -v "$var" '%s' "$val"
}

ui_input_secret() {
  local var="$1" prompt="$2" def="${3:-}"
  local val
  if ui_noninteractive; then
    val="$def"
  elif ui_gum_available; then
    if [[ -n "$def" ]]; then
      val="$(ui_gum input --password --placeholder "press Enter to keep current" --prompt "$prompt")" || return 1
      val="${val:-$def}"
    else
      val="$(ui_gum input --password --prompt "$prompt")" || return 1
    fi
  else
    onboard_prompt_secret "$var" "$prompt" "$def"
    return 0
  fi
  printf -v "$var" '%s' "$val"
}

ui_spin() {
  local title="$1"
  shift
  if ui_noninteractive; then
    "$@"
    return $?
  fi
  if ui_gum_available; then
    ui_gum spin --spinner dot --title "$title" --show-output -- "$@" || {
      ui_warn "Falling back to plain output (terminal UI unavailable)"
      echo "==> $title"
      "$@"
    }
  else
    echo "==> $title"
    "$@"
  fi
}

ui_directory_candidates() {
  local default="$1"
  local roots=("$HOME" "/mnt" "/media" "/srv")
  local r d
  printf '%s\n' "$default"
  for r in "${roots[@]}"; do
    [[ -d "$r" ]] || continue
  done
  while IFS= read -r d; do
    [[ -n "$d" ]] && printf '%s\n' "$d"
  done < <(
    for r in "${roots[@]}"; do
      [[ -d "$r" ]] || continue
      find "$r" -maxdepth 4 -type d 2>/dev/null
    done | sort -u | head -200
  )
}

ui_pick_directory() {
  local var="$1" prompt="$2" def="${3:-$HOME}"
  def="$(resolve_install_dir "$def")"
  local choice picked typed browse

  if ui_noninteractive; then
    printf -v "$var" '%s' "$def"
    return 0
  fi

  if ! ui_gum_available; then
    onboard_prompt "$var" "$prompt" "$def"
    return 0
  fi

  choice="$(ui_gum choose \
    "Type path manually" \
    "Browse folders" \
    "Pick from suggestions" \
    --header "$prompt")" || return 1

  case "$choice" in
    "Type path manually")
      ui_input typed "$prompt" "$def"
      picked="$(resolve_install_dir "$typed")"
      ;;
    "Browse folders")
      local start="$HOME"
      [[ -d "$def" ]] && start="$(dirname "$def")"
      browse="$(ui_gum file --directory "$start")" || return 1
      picked="$(resolve_install_dir "$browse")"
      ;;
    "Pick from suggestions")
      picked="$(ui_directory_candidates "$def" | ui_gum filter --height 12 --placeholder "Type to search folders…")" || return 1
      picked="$(resolve_install_dir "$picked")"
      ;;
    *)
      picked="$def"
      ;;
  esac

  [[ -n "$picked" ]] || picked="$def"
  printf -v "$var" '%s' "$picked"
}

ui_success() {
  local msg="$1"
  echo
  ui_gum_style --border rounded --padding "1 2" --foreground 42 "$msg" 2>/dev/null || echo "$msg"
  echo
}

ui_show_file() {
  local f="$1"
  if ui_noninteractive; then
    cat "$f"
    return 0
  fi
  ui_gum pager <"$f" 2>/dev/null || cat "$f"
}
