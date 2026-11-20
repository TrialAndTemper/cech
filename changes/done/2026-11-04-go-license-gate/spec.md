# Go dependency licence gate

Implemented 2026-11-20; not maintained: the code and its tests are current.

## Rules

- `mise run lint:licenses` checks the external dependencies compiled into the CECH CLI command with `go-licenses`, including transitive imports for the current platform.
  Test: by hand on 2026-11-20, in the rule 5 checks; CI runs it on Linux, macOS and Windows.
- The allowed licences are the SPDX ids listed in `scripts/check-go`, which meet [ADR 3](../../../docs/adr/0003-licence-and-contributions.md) item 4; an unlisted, missing or unidentified licence fails the gate.
  Test: by hand on 2026-11-20: with `BSD-3-Clause` taken off the list, the gate names `github.com/spf13/pflag` and exits 1.
- The checker is an exact Go tool version pinned in `mise.toml` and `mise.lock`, separate from the CLI's `go.mod` dependencies.
  Test: by hand on 2026-11-20: `mise.toml` pins `go:github.com/google/go-licenses/v2` at 2.0.1, and `cli/go.mod` does not list it.
- The checker runs from `cli/` with `GOTOOLCHAIN=local` and `GOWORK=off`, targets `./cmd/cech`, and ignores CECH's own module because its licence is at the repository root.
  Test: by hand on 2026-11-20, reading `scripts/check-go`.
- The script exits 0 on success, 1 when the checker exits 1 (licence findings or analysis errors), and 2 for setup failures or other checker exit statuses; it preserves the checker's diagnostics.
  Test: by hand on 2026-11-20: exit 0 on `main`; exit 1 with the checker's message for the finding in rule 2; exit 2 with go-licenses taken off `PATH`.
- The gate runs locally through `mise run check` and in CI through `check:os`, via the existing `lint:*` task dependency.
  Test: by hand on 2026-11-20: `mise tasks deps check` lists `lint:licenses` under `check:os`.
