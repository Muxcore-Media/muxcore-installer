# AGENTS.md — muxcore-installer

MuxCore sidecar module (`muxcore-installer`). Workspace deploy and SSH: [`../AGENTS.md`](../AGENTS.md). Default ports: [`_mvp/PORTS.md`](../_mvp/PORTS.md).

## Module identity

| Field | Value |
|-------|-------|
| Directory | `muxcore-installer` |
| Capabilities | see muxcore.json |
| Contracts | none declared |

## Agent rules

- Modules run as gRPC sidecars; capabilities are the security boundary.
- TLS required in production (`MUXCORE_INSECURE_DISABLE_TLS` is dev-only).
- Match existing Go patterns; run `gofmt` and package tests before finishing.
- Cross-module events: prefer `github.com/Muxcore-Media/contracts-media/events` over deprecated `core/pkg/contracts` aliases.
- Do not edit polluted workspace dumps (see `MASTER-ROADMAP.md` Appendix H).

## Build

```bash
cd muxcore-installer
go test ./...
```

## Homelab vault deploy (umbrella workspace)

This installer is for first-run on a single machine. If you have the full umbrella
checkout, use `_mvp/scripts/` for vault soak deploy — see [`../AGENTS.md`](../AGENTS.md).

```bash
_mvp/scripts/deploy-module-to-vault.sh --list
_mvp/scripts/deploy-module-to-vault.sh <module> --verify-all
_mvp/scripts/smoke-vault-all.sh
```
