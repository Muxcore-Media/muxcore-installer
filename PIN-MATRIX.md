# Installer pin matrix (module → tag)

Canonical machine-readable pins: [`versions.env`](versions.env).  
Spool end-user tags (`minimal`, `media`, `acquisition`, `default`, `library-plus`, `secrets-vault`) must use the **same** module versions for overlapping entries.

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
| `media-movies` | `v0.1.9` |
| `media-tvshows` | `v0.1.9` |
| `media-automation` | `v0.1.8` |
| `media-scanner` | `v0.1.9` |
| `media-custom-formats` | `v0.1.6` |
| `media-rename` | `v0.2.6` |
| `media-ffprobe` | `v0.1.8` |
| `media-subtitles` | `v0.4.8` |
| `cache-local` | `v0.1.1` |
| `ratelimit-tokenbucket` | `v0.1.2` |
| `media-root-folders` | `v0.1.6` |
| `request-media` | `v0.2.7` |
| `notification-default` | `v0.1.6` |
| `jellyfin` | `v0.2.4` |

## Spool-only (not in installer `MODULES`, still pinned)

| Module | Tag | Spool tag |
|--------|-----|-----------|
| `scheduler-cron` | `v0.1.5` | `default` |
| `cache-redis` | `v0.1.4` | `cache-redis` |
| `ratelimit-tokenbucket` | `v0.1.2` | `default` |
| `workflow-tapestry` | `v0.1.5` | `default` |
| `feature-flags-file` | `v0.1.2` | `default` |
| `media-list-sync` | `v0.1.7` | `acquisition` |
| `media-music` / `media-books` / `media-comics` / `media-audiobooks` | `v0.1.0` | `library-plus` |
| `secrets-vault` | `v0.1.1` | `secrets-vault` |

## Atomic update rule

When bumping a pin:

1. Update `versions.env` (installer).
2. Update every `spool/tags/*.json` entry for that module to the same tag.
3. Refresh this document’s tables to match.
4. Prefer one PR (or same commit batch) for installer + spool so laptop demos never mix floating/`latest` with divergent pins.

Validate locally:

```bash
./scripts/check-pin-matrix.sh
```
