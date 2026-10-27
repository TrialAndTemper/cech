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

Checked on 2026-10-28: `go.yaml.in/yaml/v3` v3.0.5 (MIT and Apache-2.0) requires no other module.
Checked on 2026-10-29: neither does `github.com/BurntSushi/toml` v1.6.0 (MIT).

## Decision

1. **Dependency budget.**
   The standard library comes first.
   A third-party module is added only for a feature the standard library lacks, when writing it would cost more than depending on it.
   Each module in `go.mod` is listed here with its reason, and adding one changes this ADR in the form [ADR 1](0001-record-architecture-decisions.md) item 5 sets.
   Tools that only build or check the CLI are pinned in `mise.toml`, not in `go.mod`.
   The list:
   - `github.com/BurntSushi/toml`: the harness data file is TOML ([ADR 9](0009-native-harness-features-first.md) item 5), as are `data/marketplace.toml` and CECH's categories file, and the standard library reads no TOML.
   - `go.yaml.in/yaml/v3`, the YAML project's maintained YAML package: `SKILL.md` and agent frontmatter are YAML, which the standard library does not read, and a parser of our own could read a description differently from the harnesses.
2. **Versions.**
   The CLI has its own SemVer versions, on the `cli/` tags of [ADR 14](0014-releases-and-versions.md) item 2.
   `cech version` prints `cech` and the version, and with `--json` also the Go release that built it and the OS and architecture it was built for.
   The CLI's release pipeline is outside this ADR.
3. **Gates.**
   The CLI's gates are tasks of [ADR 4](0004-gates-and-ci.md), run on Linux, macOS and Windows: `go vet`, golangci-lint, a format check, a tidy `go.mod`, and `go test` with the race detector where the platform allows.
   govulncheck is not a merge gate, extending [ADR 4](0004-gates-and-ci.md) item 7: it reports on pull requests without blocking them, and a failure on `main` opens an issue.
   Tests are table-driven, and output that is a contract has golden files.

## Consequences

- Every new dependency shows up as a change to this ADR.
- Release tooling must handle the `cli/` tag prefix.
- A new vulnerability report never blocks a merge; its issue on `main` asks for a fix.

## Options considered

- **A command-line framework:** a dependency for flags that the standard `flag` package handles.
- **govulncheck as a `tool` in `go.mod`:** puts its modules into `go.mod` and `go.sum`, among the binary's own dependencies.
- **govulncheck as a merge gate:** a new report would block every pull request until Go is upgraded.
