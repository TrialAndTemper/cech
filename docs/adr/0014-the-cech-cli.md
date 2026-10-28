# 14. The cech CLI

- Status: accepted
- Date: 2026-10-20
- Deciders: Michał Bożek

## Context

Earlier ADRs promise a `cech` command: `cech build` and its `--check` ([ADR 9](0009-generated-files.md)), the validator ([ADR 6](0006-claude-code-and-codex-from-one-source.md)), and the gates that run them ([ADR 10](0010-gates-and-ci.md)).
Later commands will edit the settings of Claude Code and Codex in people's repositories and home directories, where a careless edit loses someone's settings.
Claude Code and Codex are harnesses, as [ADR 3](0003-a-marketplace-you-extend-not-fork.md) defines them.
Scripts and CI read a command's output and exit status, so both become a contract as soon as someone depends on them.

Checked on 2026-10-27 with Go 1.27.3:

- Go builds a static binary for each of Linux, macOS and Windows, so users install no runtime.
- A module in a subdirectory takes its versions from tags prefixed with that directory: the module in `cli/` is versioned by `cli/vX.Y.Z` tags, and `go install` of a version records it in the binary's build information.
- `go build` stamps a version from git tags only for a module at the repository root, so a build of `cli/` records `(devel)`.
- A module at the repository root takes in every Go file below it, including any that a plugin ships, and `go vet ./...` and `go test ./...` would build them.

## Decision

1. **Go, in its own module.**
   `cli/` holds the module `github.com/TrialAndTemper/cech/cli`, with its own `go.mod`: the main package in `cmd/cech`, and everything else in `internal/`, which other modules cannot import.
   The repository root has no `go.mod`, so Go files in plugins stay out of the module, its tests and its gates.
   The module path matches the repository's path, so `go install github.com/TrialAndTemper/cech/cli/cmd/cech@<version>` works.
2. **Compose the harnesses.**
   Where Claude Code or Codex has a command for a job, `cech` runs it rather than changing the harness's state itself ([ADR 5](0005-native-tool-features-first.md)).
   It edits only settings the harnesses document.
3. **Edits preserve other bytes.**
   An edit to a file `cech` does not own changes only the keys it manages and leaves every other byte as it was.
   It refuses a file that does not parse or has duplicate keys, reads the result back to check it, and replaces the file atomically.
   Before changing a file outside the repository, it saves a backup.
4. **Writes can be previewed.**
   Every command that writes takes `--dry-run`, which reports the changes and writes nothing.
5. **Output is a contract.**
   - Results go to stdout, and messages and errors to stderr.
   - Every command except `help` takes `--json` and then prints one JSON object on stdout, also when it fails after its arguments are parsed, unless an internal error stops it (below).
     A minor release may add fields; removing a field, renaming it or changing its meaning is a breaking change.
   - Exit status: 0 done; 1 the command ran and met a failure, including a partial success; 2 a wrong command line; 3 something it needs could not be reached, so it could not decide.
     When a command both meets a failure and cannot decide something else, it exits 1, since a retry cannot fix the failure, and its output names both.
     Other codes are not used.
     Two statuses are outside `cech`'s control: the one a shell reports when a signal kills it, and the 2 that Go itself exits with, not meaning a wrong command line, after a fatal runtime error or a SIGQUIT.
   - An internal error, such as a panic in a command's code, exits 1 with its report on stderr only; stdout then holds no valid result, even with `--json`.
     A command that starts goroutines recovers a panic in each of them.
   - A command never reports success for a check it could not run, and after a partial success it says what it skipped.
6. **Harness quirks stay in one place.**
   Code that knows one harness's behaviour lives in `internal/harness/claude` or `internal/harness/codex`.
   Each workaround names its entry in `data/harnesses.toml` ([ADR 5](0005-native-tool-features-first.md)), so it can be deleted when the harness closes the gap.
7. **Works without mise.**
   A user needs neither mise nor Go to run the binary, and it never calls mise.
8. **Dependency budget.**
   The standard library comes first.
   A third-party module is added only for a feature the standard library lacks, when writing it would cost more than depending on it.
   Each module in `go.mod` is listed here with its reason, and adding one changes this ADR in the form [ADR 1](0001-record-architecture-decisions.md) item 5 sets.
   Today the list is empty.
   Tools that only build or check the CLI are pinned in `mise.toml`, not in `go.mod`.
9. **Versions.**
   The CLI has its own SemVer versions, tagged `cli/vX.Y.Z` or `cli/vX.Y.Z-rc.N`, apart from the catalogue's `v` tags.
   `cech version` prints `cech` and the version, and with `--json` also the Go release that built it and the OS and architecture it was built for.
   The version is the one a release build stamps with `-ldflags -X`, else the module version in the build information, else `0.0.0-dev`; a leading `v` is dropped.
   Builds of `cli/` get no version number from git tags, so a release build needs the stamp; the CLI's release pipeline is left to a later ADR.
10. **Gates.**
    The CLI's gates are tasks of [ADR 10](0010-gates-and-ci.md): `go vet`, golangci-lint with a linter set that includes errcheck, the gofumpt and goimports format check, a tidy `go.mod`, and `go test` with the race detector where the platform and cgo allow.
    CI runs them on Linux, macOS and Windows.
    govulncheck reads a database that changes on its own, so it is not a merge gate, extending [ADR 10](0010-gates-and-ci.md) item 8.
    It reports on pull requests that touch the CLI, and runs on `main` daily and after such pushes, where a failure opens an issue.
    Tests are table-driven, and output that is a contract has golden files.

## Consequences

- Only `cli/` has a `go.mod`, so Go commands run in `cli/`, or from the root with `-C cli`.
- Every new dependency shows up as a change to this ADR.
- Release tooling must handle the `cli/` tag prefix.
- A new vulnerability report never blocks a merge; its issue on `main` asks for a fix.

## Options considered

- **Python:** users would need a runtime.
- **Rust:** its standard library has no JSON, HTTP, SHA-256 or archive support, so the commands planned here would start with many direct and transitive crates against the dependency budget; it is also slower to build, and fewer contributors know it.
- **A module at the repository root:** takes in Go files that plugins ship.
- **A command-line framework:** a dependency for flags that the standard `flag` package handles.
- **A separate exit status for partial success:** every caller would handle one more code, and one that checks only for 1 would read a partial success as success; `--json` and stderr already say what was done and what was skipped.
- **Exit statuses as bit flags (1, 2, 4), combined when several apply:** callers would decode bits, and a script that checks for 1 would miss a combined status.
- **govulncheck as a `tool` in `go.mod`:** puts its modules into `go.mod` and `go.sum`, among the binary's own dependencies.
- **govulncheck as a merge gate:** a new report would block every pull request until Go is upgraded.
