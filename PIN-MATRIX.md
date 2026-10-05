# Installer pin matrix (module → tag)

Canonical machine-readable pins: [`versions.env`](versions.env).
Spool tags that overlap must use the same versions.

## Core

| Module | Tag |
|--------|-----|
| `core` (`muxcored`) | `v0.6.7` |

## Host stack (`MODULES` in `versions.env`)

| Module | Tag |
|--------|-----|
| `api-rest` | `v0.1.9` |
| `auth-local` | `v0.1.14` |
| `database-sqlite` | `v0.1.6` |
| `database-postgres` | `v0.1.3` |
| `secrets-file` | `v0.1.7` |
| `encryption-aesgcm` | `v0.2.8` |
| `call-policy-default` | `v0.3.6` |
| `publish-policy-default` | `v0.2.3` |
| `health-monitor` | `v0.1.6` |
| `metrics-prometheus` | `v0.1.2` |
| `tracing-otlp` | `v0.1.5` |
| `admin-ui` | `v0.1.14` |
| `metadata-tmdb` | `v0.1.6` |
| `metadata-musicbrainz` | `v0.1.0` |
| `media-movies` | `v0.1.15` |
| `media-tvshows` | `v0.1.13` |
| `media-music` | `v0.3.0` |
| `media-books` | `v0.3.1` |
| `media-comics` | `v0.2.1` |
| `media-audiobooks` | `v0.2.1` |
| `media-scanner` | `v0.1.27` |
| `media-custom-formats` | `v0.1.12` |
| `media-rename` | `v0.2.8` |
| `media-ffprobe` | `v0.1.10` |
| `media-subtitles` | `v0.5.1` |
| `cache-local` | `v0.1.2` |
| `ratelimit-tokenbucket` | `v0.1.3` |
| `media-root-folders` | `v0.1.8` |
| `media-transcoder` | `v0.3.8` |
| `media-ui` | `v0.1.0` |
| `notification-default` | `v0.1.8` |
| `jellyfin` | `v0.3.1` |
| `plex` | `v0.1.2` |
| `emby` | `v0.1.2` |
| `media-dlna` | `v0.1.1` |

The public installer does not pin or start acquisition, indexer, or downloader modules.

## Atomic update rule

When bumping a pin:

1. Update `versions.env`.
2. Update every `spool/tags/*.json` entry for that module to the same tag.
3. Refresh this document’s tables to match.

```bash
./scripts/check-pin-matrix.sh
```
