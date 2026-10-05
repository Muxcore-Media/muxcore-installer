# MuxCore installer

Single-machine first-run for a personal media library. MuxCore organizes and
plays files you already have. It does not download media.

`muxcore-setup` is a single self-contained Go binary with a
[Bubbletea](https://github.com/charmbracelet/bubbletea) TUI wizard — no bash
dependency chain, no `lib/*.sh` to keep in sync, nothing to `chmod +x` by
hand. It embeds every script and config template it needs (`up.sh`,
`bootstrap-auth.sh`, `smoke-fixture.sh`, policies, `.env.example`) and lays
them out in your chosen install folder on first run.

## One-line setup

Linux or macOS:

```bash
curl --proto '=https' --tlsv1.2 -fsSL https://getmuxcore.zem.systems | bash
```

Safer two-step (same bytes — you can read the script first):

```bash
curl --proto '=https' --tlsv1.2 -fsSL https://getmuxcore.zem.systems -o get-muxcore.sh
bash get-muxcore.sh
```

Never pipe the installer into `sudo` — it never needs root, and will refuse to run as one.

From a checkout (builds and runs `muxcore-setup` directly):

```bash
go build -o muxcore-setup ./cmd/muxcore-setup
./muxcore-setup
```

## The walkthrough

The wizard fills every screen on top; long-running steps (downloads, starting
the stack, health checks) drop a live log viewport at the bottom so you can
watch progress and any errors without losing your place.

1. **Legal agreement** — lawfully obtained media only. Scroll with the
   viewport, `y` to accept. Not skippable, even with `MUXCORE_NONINTERACTIVE`.
2. **Install folder** — default, recommended (`/opt/muxcore` if writable),
   browse, or type a path. Detects an existing install and offers
   **reconfigure** / **restart-only** / **fresh**.
3. **Runtime** — host processes or Docker Compose (with a live check for
   Docker/Compose availability), then how it should stay running: run once,
   a user `systemd --user` service, or a system service.
4. **Libraries** — Movies, TV, Music, Books, Comics, Audiobooks (multi-select).
5. **Media folders** — pick one media root, then a single table to review and
   edit every library's path plus the watch/import folder in one place
   (catches a TV folder nested under a movie folder before it becomes a mess).
6. **Playback** — MuxCore's own player and/or an existing Jellyfin / Plex /
   Emby / DLNA server, with a live connection check (spinner) as soon as you
   enter a URL and token.
7. **Network exposure** — localhost-only (default) or all interfaces for LAN
   access. The mesh between MuxCore's services is TLS (see
   [Security profile](#security-profile)); the admin and player pages are
   plain HTTP, so keep them on your home network or behind an HTTPS proxy.
8. **Admin account** — username + password, with reveal-on-demand (`r`) and
   a one-key strong-password generator (`ctrl+g`).
9. **Metadata** — offline fixture data or a live TMDB API key (validated
   against the TMDB API as you type), plus metadata language/region.
10. **Database** — SQLite (default, zero-config) or Postgres (paste a
    `DATABASE_URL`).
11. **Hardware transcoding** — auto-detected (VAAPI / NVENC / VideoToolbox);
    only asked when something was actually found.
12. **Port preflight** — checks the ports this install would bind and warns
    before you commit, instead of failing halfway through startup.
13. **Summary → install** — checklist of steps with a spinner per step and a
    progress bar for downloads, streaming logs live in the bottom viewport.

When finished, open **http://localhost:8082** and use the credentials in `run/VIEW-ME.txt`.

## Security profile

New installs run MuxCore's **household** security profile
([ADR-0016](https://github.com/Muxcore-Media/umbrella/blob/main/docs/adr/0016-security-profiles.md)):
every gRPC hop between core and the modules is TLS against core's own CA, and
every module proves who it is with a certificate core issued to it
([ADR-0017](https://github.com/Muxcore-Media/umbrella/blob/main/docs/adr/0017-mesh-module-identity.md)).
Nothing in a household install sets `MUXCORE_INSECURE_DISABLE_TLS`; core and
the modules refuse it.

| What | Where | Notes |
|------|-------|-------|
| Profile | `.env`: `MUXCORE_PROFILE=household` | `dev` = plaintext mesh (below) |
| Enrollment secret | `.env`: `MUXCORE_ENROLL_SECRET` | Random, generated once, kept on re-runs; `.env` is mode 0600. Only core receives it. |
| Module tokens | derived at start | `mct_2_<id>_` + hex(HMAC-SHA256(secret, id)), the value `bin/muxcored enroll token <id>` prints. `up.sh` hands each module its token (environment, not command line) until it has enrolled. The Compose runtime keeps them in `.env` as `MUXCORE_ENROLL_TOKEN_<ID>`. |
| Core CA | `mesh/ca/` (0700) | Core's data dir for key material (`MUXCORE_DATA_DIR=mesh`, `MUXCORE_GRPC_CA_CERT_DIR=mesh/ca`): CA key, single-use enrollment ledger (`enrolled.json`), core's server certificate. |
| Public CA | `mesh/public/ca.crt` | Exported by core (`MUXCORE_CA_EXPORT_DIR`); modules verify core with it (`MUXCORE_TLS_CA`). Core's HTTP port is HTTPS too: `curl --cacert mesh/public/ca.crt https://127.0.0.1:8080/health`. |
| Module identities | `mesh/id/<module-id>/` (0700 each) | `MUXCORE_TLS_DIR`. On first start the module generates its key, enrolls with its token and stores the certificate; later starts reuse it. |

Everything on one machine talks over loopback, which every certificate core
issues already covers, so no extra SANs are needed. For other host names set
`MUXCORE_TLS_SERVER_SANS` / `MUXCORE_ENROLL_SAN_ALLOW` in `.env` (passed to core).

**Backups ([ADR-0023](https://github.com/Muxcore-Media/umbrella/blob/main/docs/adr/0023-mesh-identity-outside-backups.md)).**
The installer has no backup job of its own. If you back up an install, back
up `data/` and `.env`, and **never `mesh/`** — it holds private keys. It sits
outside `data/` for exactly that reason (and so that Compose module containers,
which mount `data/`, cannot read the CA key). After restoring `data/` onto the
same machine, the modules keep their identities. On a new machine, start with
an empty `mesh/`: core creates a fresh CA with an empty ledger and every module
enrolls again with the same secret.

**Lost one module's identity** (deleted `mesh/id/<id>/`): its token is spent.
`up.sh` warns and prints the fix:

```bash
./bin/muxcored enroll reset <module-id> --ca-dir mesh/ca
./up.sh
```

`./bin/muxcored enroll list --ca-dir mesh/ca` shows which modules have enrolled.
**Lost `mesh/ca/`**: remove `mesh/` entirely and run `./up.sh`.

### dev profile (escape hatch)

`muxcore-setup --dev` (or `MUXCORE_PROFILE=dev`) installs the **dev** profile:
`MUXCORE_PROFILE=dev` and `MUXCORE_INSECURE_DISABLE_TLS=true` in `.env`, a
plaintext mesh and a loud warning on every start. `./up.sh --dev` runs one start
in dev whatever `.env` says. Development only — anyone who can reach the mesh
ports can impersonate a module.

### Existing installs (upgrading)

Installs made before this change run with `MUXCORE_INSECURE_DISABLE_TLS=true`
and no profile, which core treats as **dev**. Their binaries and scripts
predate household, so the installer does **not** switch them on its own: a
re-run (Reconfigure or Restart only) keeps them on dev — it pins
`MUXCORE_PROFILE=dev` so a later core release that drops the inference keeps
them working — and prints how to opt in:

```bash
muxcore-setup --household          # or MUXCORE_PROFILE=household muxcore-setup
```

Then choose **Reconfigure** or **Restart only**. Opting in stops the stack
(with the install's own `up.sh stop`), moves `bin/` to `bin.pre-household-<time>/`
and the old scripts to `*.pre-household`, installs the pinned binaries and
current scripts, removes every insecure flag from `.env`, generates the
enrollment secret and starts in household. `data/`, library paths, playback
settings and logins are kept. `MUXCORE_DRY_RUN=1` never migrates. Rollback:
`./up.sh stop`, move `bin.pre-household-*` back to `bin/` and the
`*.pre-household` files back, set `MUXCORE_PROFILE=dev` and
`MUXCORE_INSECURE_DISABLE_TLS=true` in `.env`.

### Admin login over HTTP

In household, admin-ui marks its session cookie `Secure`. Browsers accept that
on `http://localhost:8082`, but not on `http://<lan-ip>:8082`: for LAN access,
put the admin UI behind an HTTPS reverse proxy (or give admin-ui
`ADMIN_UI_TLS_CERT`/`ADMIN_UI_TLS_KEY`).

## Non-interactive / CI

```bash
MUXCORE_I_AGREE=1 MUXCORE_NONINTERACTIVE=1 MUXCORE_DRY_RUN=1 ./muxcore-setup
```

`MUXCORE_NONINTERACTIVE=1` alone is **not** consent — you must also set
`MUXCORE_I_AGREE=1`. Other env vars mirror the wizard's questions:
`MUXCORE_INSTALL_DIR`, `MUXCORE_LIBRARIES` (CSV), `MUXCORE_PLAYBACK` (CSV),
`INSTALL_RUNTIME`, `MUXCORE_KEEP_MODE`, `MUXCORE_DB_BACKEND`, `DATABASE_URL`,
`MVP_ADMIN_USER`/`MVP_ADMIN_PASSWORD`, `TMDB_API_KEY`,
`MUXCORE_METADATA_LANGUAGE`, `JELLYFIN_BASE_URL`/`JELLYFIN_API_KEY`,
`PLEX_URL`/`PLEX_TOKEN`, `EMBY_URL`/`EMBY_TOKEN`, `MUXCORE_BIND_ALL`,
`MUXCORE_PROFILE` (`household` | `dev`, same as `--household` / `--dev`). See
`muxcore-setup --help`.

## Prerequisites

| Tool | Required? |
|------|-----------|
| A terminal | Yes, for the interactive wizard (non-interactive mode needs none) |
| `ffmpeg` / `ffprobe` | Optional — asked if you enable the MuxCore player or video libraries |
| Docker / Podman | Optional — asked if you pick Compose or a local Postgres container |
| Go | Only to build `muxcore-setup` from source — releases ship a static binary |
| GitHub token | Only if module releases are still private (closed alpha). The installer tries public downloads first, then prompts. |

Supported OS/arch: linux/darwin × amd64/arm64. Native Windows is not supported; use WSL2.

Regenerate the ASCII splash:

```bash
./scripts/gen-banner.sh   # uses nix-shell -p figlet
```

## Manual path (skip the wizard)

Point `muxcore-setup` at an empty directory once (`MUXCORE_DRY_RUN=1` — it
extracts `up.sh`, `bootstrap-auth.sh`, `smoke-fixture.sh`, `.env`, etc. and
stops), then drive the scripts directly:

```bash
MUXCORE_I_AGREE=1 MUXCORE_NONINTERACTIVE=1 MUXCORE_DRY_RUN=1 ./muxcore-setup
./up.sh
./bootstrap-auth.sh
./smoke-fixture.sh
./up.sh stop
```

`bootstrap-auth.sh` logs in through auth-local's `POST /login/device` (auth-local
creates the admin from `MVP_ADMIN_USER`/`MVP_ADMIN_PASSWORD` on an empty
database) and writes `run/admin.token`. `smoke-fixture.sh` checks core over
HTTPS in household, that core reports the profile `.env` asks for, and that
every running module has a certificate in `mesh/id/`.

## Default modules

The wizard starts a platform core plus the libraries you selected. It does **not**
install downloaders, indexers, or request/automation pipelines.

| Layer | Modules |
|-------|---------|
| Platform | `muxcored`, `api-rest`, `auth-local`, `database-sqlite` (or postgres), `secrets-file`, `encryption-aesgcm`, `call-policy-default`, `publish-policy-default`, `cache-local`, `ratelimit-tokenbucket`, `health-monitor`, `admin-ui`, `notification-default` |
| Libraries | `media-movies`, `media-tvshows`, `media-music`, `media-books`, `media-comics`, `media-audiobooks` (as selected) plus scanner, root folders, and video helpers |

Pins: [`versions.env`](versions.env). Matrix: [`PIN-MATRIX.md`](PIN-MATRIX.md).

## Homelab / umbrella developers

If you maintain the full MuxCore module workspace (module submodules, vault soak
stack), use the umbrella repo's [`AGENTS.md`](https://github.com/Muxcore-Media/umbrella/blob/main/AGENTS.md)
instead of this installer for day-to-day deploy:

| Task | Command |
|------|---------|
| List deploy targets | `_mvp/scripts/deploy-module-to-vault.sh --list` |
| Deploy one module to vault | `_mvp/scripts/deploy-module-to-vault.sh <module> --verify-all` |
| SSH preflight (before deploy) | `_mvp/scripts/preflight-vault-ssh.sh` |
| Full vault smoke | `_mvp/scripts/smoke-vault-all.sh` |
| Vault mesh CLI | `_mvp/scripts/muxcorectl-vault.sh health status` |
| Local / vault HTTP smoke | `_mvp/scripts/smoke-vault-health.sh` |
| Public edge smoke | `_mvp/scripts/smoke-vault-public.sh` |
| Local stack status | `cd _mvp && ./run-host.sh status` |
| Remove stale pidfiles | `cd _mvp && ./run-host.sh cleanup-stale` |
| Script tests (offline) | `_mvp/scripts/run-script-tests.sh` |

The installer targets **first-run on a single machine**; the umbrella `_mvp/run-host.sh`
path is the developer soak stack on vault and local Nix workstations.

## Layout

| Path | Role |
|------|------|
| `cmd/muxcore-setup/` | Entry point — interactive TUI (`main.go`) and env-driven non-interactive mode (`noninteractive.go`) |
| `internal/tui/` | Bubbletea model, phases (one file per wizard screen), styles, and widgets |
| `internal/wizard/` | `Answers` (collected choices) and `Pipeline` (configure → fetch → seed → start → health → admin → smoke) |
| `internal/assets/` | Embedded copies of `up.sh`, `bootstrap-auth.sh`, `smoke-fixture.sh`, `muxcore.json`, `.env.example`, `lib/`, `policies/` — the repo-root copies are symlinks into here so there is one source of truth |
| `internal/mesh/` | Security profile resolution, enrollment tokens (core's algorithm) and the `mesh/` key-material layout |
| `internal/{pathutil,envfile,ghrelease,fetch,seedroots,compose,prereqs,execstream,pins,modules}/` | Ported logic from the old `lib/*.sh` (paths, `.env` read/write, GitHub release downloads, module resolution, SQLite root-folder seeding, Compose file generation, OS/hardware detection, subprocess log streaming, version pins) |
| `get-onboard.sh` | One-liner entry — downloads the pinned `muxcore-setup` binary for your OS/arch and execs it |
| `scripts/build-release.sh` | Cross-compiles `muxcore-setup` for linux/darwin × amd64/arm64 and packages the tarballs `get-onboard.sh` expects |

Developer-only: `MUXCORE_LAB_BIN` copies unpublished binaries from a local `bin/` directory. The public wizard does not mention this.

## GitHub releases

Module binaries come from GitHub Releases at
`https://github.com/Muxcore-Media/<module>/releases`. Every tarball is verified
against the release `SHA256SUMS`. For private assets, GitHub tokens
(`GITHUB_TOKEN`, `GH_TOKEN`, `MUXCORE_GITHUB_TOKEN`, or
`~/.config/muxcore/github.token`) are used.

`muxcore-setup` itself is published the same way — build with
`scripts/build-release.sh` and upload with `scripts/publish-github-release-assets.sh`
so `get-onboard.sh` has something to fetch.
