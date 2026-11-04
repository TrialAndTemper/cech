# Go dependency licence gate

## Rules

- `mise run check` checks the licenses of the external dependencies of the CECH CLI command with `go-licenses`, ignoring CECH's own module because its license is at the repository root.
- The checker is an exact Go tool version pinned in `mise.toml` and `mise.lock`, separate from the CLI's `go.mod` dependencies.
- The gate allows only the licences in ADR 23 item 2 and rejects any other or unknown licence, reporting tool execution failures distinctly from findings.
- The gate runs locally and in CI through the existing `mise run check` entry point.
