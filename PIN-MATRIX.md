# Installer pin matrix (module → tag)

Canonical machine-readable pins: [`versions.env`](versions.env).
Spool tags that overlap must use the same versions.

## Core

| Module | Tag |
|--------|-----|
| `core` (`muxcored`) | `v0.6.15` |

## Host stack (`MODULES` in `versions.env`)

| Module | Tag |
|--------|-----|
| `api-rest` | `v0.1.12` |
| `auth-local` | `v0.1.18` |
| `database-sqlite` | `v0.1.8` |
| `database-postgres` | `v0.1.5` |
| `secrets-file` | `v0.1.9` |
| `encryption-aesgcm` | `v0.2.10` |
| `call-policy-default` | `v0.3.8` |
| `publish-policy-default` | `v0.2.5` |
| `health-monitor` | `v0.1.9` |
| `metrics-prometheus` | `v0.1.4` |
| `tracing-otlp` | `v0.1.7` |
| `admin-ui` | `v0.1.20` |
| `metadata-tmdb` | `v0.1.9` |
| `metadata-musicbrainz` | `v0.1.2` |
| `media-movies` | `v0.1.20` |
| `media-tvshows` | `v0.1.18` |
| `media-music` | `v0.3.2` |
| `media-books` | `v0.3.3` |
| `media-comics` | `v0.2.3` |
| `media-audiobooks` | `v0.2.3` |
| `media-scanner` | `v0.1.32` |
| `media-custom-formats` | `v0.1.15` |
| `media-rename` | `v0.2.11` |
| `media-ffprobe` | `v0.1.13` |
| `media-subtitles` | `v0.5.4` |
| `cache-local` | `v0.1.4` |
| `ratelimit-tokenbucket` | `v0.1.5` |
| `media-root-folders` | `v0.1.11` |
| `media-transcoder` | `v0.3.10` |
| `media-ui` | `v0.1.0` |
| `notification-default` | `v0.1.10` |
| `jellyfin` | `v0.3.5` |
| `plex` | `v0.1.5` |
| `emby` | `v0.1.5` |
| `media-dlna` | `v0.1.4` |

The public installer does not pin or start acquisition, indexer, or downloader modules.

## Atomic update rule

When bumping a pin:

1. Update `versions.env`.
2. Update every `spool/tags/*.json` entry for that module to the same tag.
3. Refresh this document’s tables to match.

```bash
./scripts/check-pin-matrix.sh
```
