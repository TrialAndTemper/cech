# 16. Building the cech CLI

- Status: accepted
- Date: 2026-10-29
- Deciders: Michał Bożek

## Context

The `cech` CLI is a Go module in `cli/` ([ADR 15](0015-the-cech-cli.md) item 1).
CECH ships every third-party module the CLI uses, and every module those need, and must keep them all current.
CECH releases its layout rules with each CLI release, separately from its catalogue releases ([ADR 8](0008-a-marketplace-you-extend-not-fork.md) item 6), so users and scripts need to know which CLI they run.
A vulnerability database changes on its own, so a check against it can fail a branch that has not changed.

Checked on 2026-10-27 with Go 1.27.3: a module in a subdirectory takes its versions from tags prefixed with that directory, so the module in `cli/` is versioned by `cli/vX.Y.Z` tags, and `go install` of a version records it in the binary's build information.

Checked on 2026-10-31 with Go 1.27.3: of the modules `github.com/spf13/cobra` v1.10.2 (Apache-2.0) requires, only two are built into the CLI: `github.com/spf13/pflag` v1.0.10 (BSD-3-Clause) and `github.com/inconshreveable/mousetrap` v1.1.0 (Apache-2.0), which only Windows builds use.

## Decision

1. **Dependency budget.**
   The standard library comes first.
   A third-party module is added only for a feature the standard library lacks, when writing it would cost more than depending on it.
   Each module the CLI requires directly says on its line in `go.mod` why it is there, and the commit that adds it records its licence.
   External dependencies compiled into the CLI may use only these licences, by SPDX id: `MIT`, `BSD-2-Clause`, `BSD-3-Clause`, `ISC`, `0BSD`, `Apache-2.0`, `CC0-1.0`.
   Tools that only build or check the CLI are pinned in `mise.toml`, not in `go.mod`.
   The command line is built on `github.com/spf13/cobra`, since the standard `flag` package has no command tree, generated help or shell completion.
2. **Versions.**
   The CLI has its own SemVer versions, on the `cli/` tags of [ADR 14](0014-releases-and-versions.md) item 2.
   `cech version` reports the CLI's version, so users and scripts can tell which layout rules they run.
   The CLI's release pipeline is outside this ADR.
3. **Gates.**
   A licence gate checks the external dependencies compiled into the CLI command, including transitive imports, against item 1's list.
   It runs for the current platform locally and on each CI platform, so platform-specific imports are checked too ([ADR 4](0004-gates-and-ci.md) item 2).
   govulncheck is not a gate, extending [ADR 4](0004-gates-and-ci.md) item 7: it reports on pull requests without blocking them, and a failure on `main` opens an issue.
   Output that is a contract has golden files.

## Consequences

- Every new dependency shows up in `go.mod` with its reason.
- Release tooling must handle the `cli/` tag prefix.
- A new vulnerability report never blocks a merge; its issue on `main` asks for a fix.

## Options considered

- **The standard `flag` package and a command table of our own:** no new modules, but each command adds its own parsing and help code, and shell completion is ours to write.
- **govulncheck as a `tool` in `go.mod`:** puts its modules into `go.mod` and `go.sum`, among the binary's own dependencies.
- **govulncheck as a gate:** a new report would block every pull request until Go is upgraded.
