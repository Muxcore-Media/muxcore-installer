# Path picker: default / recommended / browse / type.
# shellcheck shell=bash

# paths_recommended_install
#   default: $HOME/muxcore
#   recommended: /opt/muxcore when writable, else $HOME/muxcore
paths_install_default() {
  printf '%s\n' "${MUXCORE_INSTALL_DIR:-$HOME/muxcore}"
}

paths_install_recommended() {
  if [[ -d /opt && -w /opt ]]; then
    printf '%s\n' /opt/muxcore
    return 0
  fi
  if [[ -d /opt/muxcore && -w /opt/muxcore ]]; then
    printf '%s\n' /opt/muxcore
    return 0
  fi
  printf '%s\n' "$HOME/muxcore"
}

paths_media_default() {
  printf '%s\n' "$HOME/Media"
}

paths_media_recommended() {
  if [[ -d /mnt/media ]]; then
    printf '%s\n' /mnt/media
    return 0
  fi
  if [[ "$(uname -s)" == "Darwin" && -d "$HOME/Movies" ]]; then
    printf '%s\n' "$HOME/Movies"
    return 0
  fi
  if [[ -d "$HOME/Videos" ]]; then
    printf '%s\n' "$HOME/Videos"
    return 0
  fi
  printf '%s\n' "$HOME/Media"
}

# ui_pick_dir VAR prompt default recommended
ui_pick_dir() {
  local var="$1" prompt="$2" def="$3" rec="${4:-}"
  def="$(resolve_install_dir "$def")"
  [[ -n "$rec" ]] && rec="$(resolve_install_dir "$rec")"
  choice=""; picked=""; typed=""; browse=""; start=""

  if ui_noninteractive; then
    printf -v "$var" '%s' "$def"
    return 0
  fi

  ui_detect_backend
  if [[ -n "$rec" && "$rec" != "$def" ]]; then
    ui_choose choice \
      "Use default ($def)" \
      "Use recommended ($rec)" \
      "Browse folders" \
      "Type a path" \
      --header "$prompt"
  else
    ui_choose choice \
      "Use default ($def)" \
      "Browse folders" \
      "Type a path" \
      --header "$prompt"
  fi

  case "$choice" in
    "Use default"*)
      picked="$def"
      ;;
    "Use recommended"*)
      picked="${rec:-$def}"
      ;;
    "Browse folders")
      start="$HOME"
      [[ -d "$def" ]] && start="$(dirname "$def")"
      [[ -d "$start" ]] || start="$HOME"
      case "${UI_BACKEND:-}" in
        gum)
          browse="$(ui_gum file --directory "$start")" || return 1
          picked="$(resolve_install_dir "$browse")"
          ;;
        dialog)
          browse="$(dialog --dselect "$start/" 18 70 3>&1 1>&2 2>&3)" || return 1
          picked="$(resolve_install_dir "$browse")"
          ;;
        *)
          ui_input typed "$prompt (tab-complete)" "$def"
          picked="$(resolve_install_dir "$typed")"
          ;;
      esac
      ;;
    "Type a path")
      ui_input typed "$prompt (tab-complete)" "$def"
      picked="$(resolve_install_dir "$typed")"
      ;;
    *)
      picked="$def"
      ;;
  esac

  [[ -n "$picked" ]] || picked="$def"
  printf -v "$var" '%s' "$picked"
}

# Confirm a derived per-kind path, allowing override.
paths_confirm_kind() {
  local var="$1" kind="$2" derived="$3"
  derived="$(resolve_install_dir "$derived")"
  if ui_noninteractive; then
    printf -v "$var" '%s' "$derived"
    return 0
  fi
  choice=""
  ui_choose choice \
    "Use $derived" \
    "Choose a different folder" \
    --header "$kind library folder"
  if [[ "$choice" == "Choose a different folder" ]]; then
    ui_pick_dir "$var" "$kind library folder" "$derived" "$derived"
  else
    printf -v "$var" '%s' "$derived"
  fi
}
