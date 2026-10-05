# AGENTS.md — muxcore-installer

MuxCore **household first-run installer** — a self-contained Go binary (`muxcore-setup`) with a Bubbletea TUI wizard and a non-interactive env-driven pipeline. It is **not** a gRPC sidecar module.

Workspace deploy and SSH (vault soak stack): [`../AGENTS.md`](../AGENTS.md).

## What this repo is

| Artifact | Role |
|----------|------|
| `cmd/muxcore-setup/` | Entry — interactive TUI + `MUXCORE_NONINTERACTIVE=1` pipeline |
| `get-onboard.sh` | One-liner bootstrap — downloads pinned `muxcore-setup` from GitHub Releases |
| `internal/wizard/` | `Answers` + `Pipeline` (configure → fetch → seed → start → health → admin → smoke) |
| `internal/assets/` | Embedded `up.sh`, `bootstrap-auth.sh`, `.env.example`, policies |
| `versions.env` | Module pin matrix (embedded at build time) |

Release origin: **GitHub Releases** (`https://github.com/Muxcore-Media/<repo>/releases`); GitHub is the sole origin.

## Build & test

```bash
cd muxcore-installer
go test ./...
go vet ./...
gofmt -l .

# Non-interactive dry-run (CI gate)
go build -o muxcore-setup ./cmd/muxcore-setup
MUXCORE_I_AGREE=1 MUXCORE_NONINTERACTIVE=1 MUXCORE_DRY_RUN=1 ./muxcore-setup
```

CI: `.github/workflows/ci.yml`.

## Agent rules

- Household installer targets **single-machine first-run**, not the vault `_mvp/run-host.sh` soak stack.
- Module binaries and `media-ui-app` dist fetch from GitHub Releases; verify every tarball against release `SHA256SUMS`.
- Never default `MVP_ADMIN_PASSWORD` to `admin-dev-only` — generate 16 chars or require explicit env.
- `restart-only` must load existing `.env` and must not clobber library paths, playback, or admin creds.
- Pins: `versions.env` / `PIN-MATRIX.md`. Do not edit polluted workspace dumps (`MASTER-ROADMAP.md` Appendix H).

## Homelab developers

For vault soak deploy of individual modules, use umbrella `_mvp/scripts/` — see [`../AGENTS.md`](../AGENTS.md).
