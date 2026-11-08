# 16. Building the cech CLI

- Status: accepted
- Date: 2026-10-29
- Deciders: Michał Bożek

## Context

The `cech` CLI is a Go module in `cli/` ([ADR 15](0015-the-cech-cli.md) item 1).
CECH ships every third-party module the CLI uses, and every module those need, and must keep them all current.
CECH releases its layout rules with each CLI release, separately from its catalogue releases ([ADR 8](0008-a-marketplace-you-extend-not-fork.md) item 6), so users and scripts need to know which CLI they run.
A vulnerability database changes on its own, so a check against it can fail a branch that has not changed.

Checked on 2026-10-27 with Go 1.27.3: the module in `cli/` is versioned by `cli/` tags, and `go install` records the version in the binary.

Checked on 2026-10-31 with Go 1.27.3: cobra v1.10.2 builds two more modules into the CLI, one only on Windows, all under permissive licences.

## Decision

1. **Dependency budget.**
   The standard library comes first.
   A third-party module is added only for a feature the standard library lacks, when writing it would cost more than depending on it.
   Each module the CLI requires directly states why it is there, and every module compiled into the CLI has a licence that [ADR 2](0002-licence-and-contributions.md) item 4 accepts.
   Tools that only build or check the CLI are pinned in `mise.toml`, not in `go.mod`.
   The command line is built on `github.com/spf13/cobra`, since the standard `flag` package has no command tree, generated help or shell completion.
2. **Versions.**
   The CLI has its own SemVer versions, on the `cli/` tags of [ADR 14](0014-releases-and-versions.md) item 2.
   `cech version` reports the CLI's version, so users and scripts can tell which layout rules they run.
3. **Gates.**
   A licence gate checks every module compiled into the CLI, including transitive ones, on each platform.
   govulncheck is not a gate, extending [ADR 4](0004-gates-and-ci.md) item 6: it reports on pull requests without blocking them, and a failure on `main` opens an issue.

## Consequences

- Every new dependency shows up in `go.mod` with its reason.
- Release tooling must handle the `cli/` tag prefix.
- A new vulnerability report never blocks a merge; its issue on `main` asks for a fix.

## Options considered

- **The standard `flag` package and a command table of our own:** no new modules, but each command adds its own parsing and help code, and shell completion is ours to write.
- **govulncheck as a `tool` in `go.mod`:** puts its modules into `go.mod` and `go.sum`, among the binary's own dependencies.
- **govulncheck as a gate:** a new report would block every pull request until Go is upgraded.
