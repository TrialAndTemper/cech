# 22. Building the cech CLI

- Status: accepted
- Date: 2026-10-29
- Deciders: Michał Bożek

## Context

The `cech` CLI is a Go module in `cli/` ([ADR 14](0014-the-cech-cli.md) item 1).
Each third-party module it uses is code CECH ships and must keep current, together with the modules that one needs.
CECH releases its layout rules with each CLI release, apart from its catalogue releases ([ADR 3](0003-a-marketplace-you-extend-not-fork.md) item 6), so users and scripts need to know which CLI they run.
A vulnerability database changes on its own, so a check against it can fail a branch that has not changed.

Checked on 2026-10-27 with Go 1.27.3: a module in a subdirectory takes its versions from tags prefixed with that directory, so the module in `cli/` is versioned by `cli/vX.Y.Z` tags, and `go install` of a version records it in the binary's build information.

Checked on 2026-10-29: `github.com/BurntSushi/toml` v1.6.0 (MIT) requires no other module.

## Decision

1. **Dependency budget.**
   The standard library comes first.
   A third-party module is added only for a feature the standard library lacks, when writing it would cost more than depending on it.
   Each module in `go.mod` is listed here with its reason, and adding one changes this ADR in the form [ADR 1](0001-record-architecture-decisions.md) item 5 sets.
   Tools that only build or check the CLI are pinned in `mise.toml`, not in `go.mod`.
   The list:
   - `github.com/BurntSushi/toml`: the harness data file is TOML ([ADR 5](0005-native-tool-features-first.md) item 5), and the standard library reads no TOML.
2. **Versions.**
   The CLI has its own SemVer versions, tagged `cli/vX.Y.Z` or `cli/vX.Y.Z-rc.N`, apart from the catalogue's `v` tags.
   `cech version` prints `cech` and the version, and with `--json` also the Go release that built it and the OS and architecture it was built for.
   The CLI's release pipeline is outside this ADR.
3. **Gates.**
   The CLI's gates are tasks of [ADR 10](0010-gates-and-ci.md), run on Linux, macOS and Windows: `go vet`, golangci-lint, a format check, a tidy `go.mod`, and `go test` with the race detector where the platform allows.
   govulncheck is not a merge gate, extending [ADR 10](0010-gates-and-ci.md) item 7; it runs outside them, and a failure on `main` opens an issue.
   Tests are table-driven, and output that is a contract has golden files.

## Consequences

- Every new dependency shows up as a change to this ADR.
- Release tooling must handle the `cli/` tag prefix.
- A new vulnerability report never blocks a merge; its issue on `main` asks for a fix.

## Options considered

- **A command-line framework:** a dependency for flags that the standard `flag` package handles.
- **govulncheck as a `tool` in `go.mod`:** puts its modules into `go.mod` and `go.sum`, among the binary's own dependencies.
- **govulncheck as a merge gate:** a new report would block every pull request until Go is upgraded.
