# Installer pin matrix (module → tag)

Canonical machine-readable pins: [`versions.env`](versions.env).
Spool tags that overlap must use the same versions.

## Core

| Module | Tag |
|--------|-----|
| `core` (`muxcored`) | `v0.5.0` |

## Host stack (`MODULES` in `versions.env`)

| Module | Tag |
|--------|-----|
| `api-rest` | `v0.1.7` |
| `auth-local` | `v0.1.5` |
| `database-sqlite` | `v0.1.5` |
| `database-postgres` | `v0.1.2` |
| `secrets-file` | `v0.1.6` |
| `encryption-aesgcm` | `v0.2.6` |
| `call-policy-default` | `v0.3.3` |
| `publish-policy-default` | `v0.2.2` |
| `health-monitor` | `v0.1.5` |
| `metrics-prometheus` | `v0.1.1` |
| `tracing-otlp` | `v0.1.4` |
| `admin-ui` | `v0.1.10` |
| `metadata-tmdb` | `v0.1.5` |
| `metadata-musicbrainz` | `v0.1.0` |
| `media-movies` | `v0.1.9` |
| `media-tvshows` | `v0.1.9` |
| `media-music` | `v0.1.0` |
| `media-books` | `v0.1.0` |
| `media-comics` | `v0.1.0` |
| `media-audiobooks` | `v0.1.0` |
| `media-scanner` | `v0.1.9` |
| `media-custom-formats` | `v0.1.6` |
| `media-rename` | `v0.2.6` |
| `media-ffprobe` | `v0.1.8` |
| `media-subtitles` | `v0.4.8` |
| `cache-local` | `v0.1.1` |
| `ratelimit-tokenbucket` | `v0.1.2` |
| `media-root-folders` | `v0.1.6` |
| `media-transcoder` | `v0.1.0` |
| `media-ui` | `v0.1.0` |
| `notification-default` | `v0.1.6` |
| `jellyfin` | `v0.2.4` |
| `plex` | `v0.1.0` |
| `emby` | `v0.1.0` |
| `media-dlna` | `v0.1.0` |

The public installer does not pin or start acquisition, indexer, or downloader modules.

## Atomic update rule

When bumping a pin:

1. Update `versions.env`.
2. Update every `spool/tags/*.json` entry for that module to the same tag.
3. Refresh this document’s tables to match.

```bash
./scripts/check-pin-matrix.sh
```
