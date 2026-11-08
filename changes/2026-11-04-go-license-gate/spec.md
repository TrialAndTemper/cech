# Go dependency licence gate

## Rules

- `mise run lint:licenses` checks the external dependencies compiled into the CECH CLI command with `go-licenses`, including transitive imports for the current platform.
- The allowed licences are the SPDX ids listed in `scripts/check-go`, which meet [ADR 3](../../docs/adr/0003-licence-and-contributions.md) item 4; an unlisted, missing or unidentified licence fails the gate.
- The checker is an exact Go tool version pinned in `mise.toml` and `mise.lock`, separate from the CLI's `go.mod` dependencies.
- The checker runs from `cli/` with `GOTOOLCHAIN=local` and `GOWORK=off`, targets `./cmd/cech`, and ignores CECH's own module because its licence is at the repository root.
- The script exits 0 on success, 1 when the checker exits 1 (licence findings or analysis errors), and 2 for setup failures or other checker exit statuses; it preserves the checker's diagnostics.
- The gate runs locally through `mise run check` and in CI through `check:os`, via the existing `lint:*` task dependency.
