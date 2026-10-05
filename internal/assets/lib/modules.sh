# Resolve ENABLED_MODULES from wizard answers. Acquisition modules do not exist.
# shellcheck shell=bash

PLATFORM_MODULES="muxcored api-rest auth-local secrets-file encryption-aesgcm call-policy-default publish-policy-default cache-local ratelimit-tokenbucket health-monitor admin-ui notification-default"

# Never deploy these from the public installer.
ACQUISITION_NEVER="media-automation request-media downloader-native-torrent downloader-native-usenet downloader-qbittorrent downloader-sabnzbd downloader-debrid indexer-piratebay indexer-torznab"

module_enabled() {
  case " ${ENABLED_MODULES:-} " in
    *" $1 "*) return 0 ;;
    *) return 1 ;;
  esac
}

csv_has() {
  local csv="$1" needle="$2"
  case ",$csv," in
    *",$needle,"*) return 0 ;;
    *) return 1 ;;
  esac
}

# resolve_enabled_modules libraries_csv playback_csv profile
# libraries_csv: Movies,TV,Music,Books,Comics,Audiobooks
# playback_csv: muxcore-player,jellyfin,plex,emby,dlna
resolve_enabled_modules() {
  local libraries="$1" playback="$2" profile="${3:-sqlite}"
  local out="$PLATFORM_MODULES"
  if [[ "$profile" == postgres ]]; then
    out="$out database-postgres"
  else
    out="$out database-sqlite"
  fi

  local any_lib=0 video=0
  csv_has "$libraries" Movies && { out="$out media-movies"; any_lib=1; video=1; }
  csv_has "$libraries" TV && { out="$out media-tvshows"; any_lib=1; video=1; }
  csv_has "$libraries" Music && { out="$out media-music metadata-musicbrainz"; any_lib=1; }
  csv_has "$libraries" Books && { out="$out media-books"; any_lib=1; }
  csv_has "$libraries" Comics && { out="$out media-comics"; any_lib=1; }
  csv_has "$libraries" Audiobooks && { out="$out media-audiobooks"; any_lib=1; }

  if [[ "$any_lib" -eq 1 ]]; then
    out="$out media-scanner media-root-folders"
  fi
  if [[ "$video" -eq 1 ]]; then
    out="$out metadata-tmdb media-rename media-ffprobe media-subtitles media-custom-formats"
  fi

  csv_has "$playback" "MuxCore player" && out="$out media-transcoder mediauiprox"
  csv_has "$playback" Jellyfin && out="$out jellyfin"
  csv_has "$playback" Plex && out="$out plex"
  csv_has "$playback" Emby && out="$out emby"
  csv_has "$playback" DLNA && out="$out media-dlna"

  # Strip anything that must never ship.
  local cleaned="" m
  for m in $out; do
    case " $ACQUISITION_NEVER " in
      *" $m "*) continue ;;
    esac
    case " $cleaned " in
      *" $m "*) continue ;;
    esac
    cleaned="$cleaned $m"
  done
  cleaned="${cleaned#"${cleaned%%[![:space:]]*}"}"
  printf '%s\n' "$cleaned"
}

module_tag() {
  local name="$1" line repo tag
  if [[ "$name" == muxcored ]]; then
    printf '%s\n' "${CORE_TAG:-v0.6.15}"
    return 0
  fi
  while IFS= read -r line; do
    [[ -z "$line" || "$line" =~ ^# ]] && continue
    repo="${line%%=*}"
    tag="${line#*=}"
    if [[ "$repo" == "$name" ]]; then
      printf '%s\n' "$tag"
      return 0
    fi
  done <<EOF
${MODULES:-}
EOF
  printf '%s\n' "v0.1.0"
}
