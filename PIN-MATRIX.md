# Installer pin matrix (module → tag)

Canonical machine-readable pins: [`versions.env`](versions.env).
Spool tags that overlap must use the same versions.

## Core

| Module | Tag |
|--------|-----|
| `core` (`muxcored`) | `v0.6.13` |

## Host stack (`MODULES` in `versions.env`)

| Module | Tag |
|--------|-----|
| `api-rest` | `v0.1.10` |
| `auth-local` | `v0.1.17` |
| `database-sqlite` | `v0.1.7` |
| `database-postgres` | `v0.1.4` |
| `secrets-file` | `v0.1.8` |
| `encryption-aesgcm` | `v0.2.9` |
| `call-policy-default` | `v0.3.7` |
| `publish-policy-default` | `v0.2.4` |
| `health-monitor` | `v0.1.8` |
| `metrics-prometheus` | `v0.1.3` |
| `tracing-otlp` | `v0.1.6` |
| `admin-ui` | `v0.1.17` |
| `metadata-tmdb` | `v0.1.7` |
| `metadata-musicbrainz` | `v0.1.1` |
| `media-movies` | `v0.1.18` |
| `media-tvshows` | `v0.1.16` |
| `media-music` | `v0.3.1` |
| `media-books` | `v0.3.2` |
| `media-comics` | `v0.2.2` |
| `media-audiobooks` | `v0.2.2` |
| `media-scanner` | `v0.1.30` |
| `media-custom-formats` | `v0.1.13` |
| `media-rename` | `v0.2.9` |
| `media-ffprobe` | `v0.1.11` |
| `media-subtitles` | `v0.5.2` |
| `cache-local` | `v0.1.3` |
| `ratelimit-tokenbucket` | `v0.1.4` |
| `media-root-folders` | `v0.1.9` |
| `media-transcoder` | `v0.3.9` |
| `media-ui` | `v0.1.0` |
| `notification-default` | `v0.1.9` |
| `jellyfin` | `v0.3.3` |
| `plex` | `v0.1.3` |
| `emby` | `v0.1.3` |
| `media-dlna` | `v0.1.2` |

The public installer does not pin or start acquisition, indexer, or downloader modules.

## Atomic update rule

When bumping a pin:

1. Update `versions.env`.
2. Update every `spool/tags/*.json` entry for that module to the same tag.
3. Refresh this document’s tables to match.

```bash
./scripts/check-pin-matrix.sh
```
