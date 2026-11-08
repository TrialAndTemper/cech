# 15. The cech CLI

- Status: accepted
- Date: 2026-10-20
- Deciders: Michał Bożek

## Context

Earlier ADRs promise a `cech` command: the validator ([ADR 10](0010-claude-code-and-codex-from-one-source.md)), the build of generated files, and the gates that run them ([ADR 4](0004-gates-and-ci.md)).
Some commands edit the harnesses' settings in people's repositories and home directories, where a careless edit loses someone's settings.
Scripts and CI read a command's output and exit status, so both become a contract as soon as someone depends on them.

Checked on 2026-10-27 with Go 1.27.3:

- Go builds a static binary for each of Linux, macOS and Windows, so users install no runtime.
- A module at the repository root takes in every Go file below it, including any that a plugin ships, and `go vet ./...` and `go test ./...` would build them.

## Decision

1. **Go, in its own module.**
   `cli/` holds the module `github.com/TrialAndTemper/cech/cli`, with its own `go.mod`: the main package in `cmd/cech`, and everything else in `internal/`, which other modules cannot import.
   The repository root has no `go.mod`, so Go files in plugins stay out of the module, its tests and its gates.
   The module path matches the repository's path, so `go install github.com/TrialAndTemper/cech/cli/cmd/cech@<version>` works.
2. **Compose the harnesses.**
   Where Claude Code or Codex has a command for a job, `cech` runs it rather than changing the harness's state itself ([ADR 9](0009-native-harness-features-first.md)).
   It edits only settings the harnesses document.
3. **Edits preserve other bytes.**
   An edit to a file `cech` does not own changes only the keys it manages and leaves every other byte as it was.
   It refuses a file that does not parse or has duplicate keys, reads the result back to check it, and replaces the file atomically.
   Before changing a file outside the repository, it saves a backup.
4. **Writes can be previewed.**
   Every command that writes can report its changes without making them.
5. **Output is a contract.**
   Results go to stdout and messages to stderr; on request, a command prints one JSON object instead, and it exits with one of a small, fixed set of statuses, with partial success counted as a failure.
   A command never reports success for a check it could not run.
   A minor release may add JSON fields; removing, renaming or changing the meaning of a field or a status is a breaking change.
6. **Harness quirks stay in one place.**
   Code that knows one harness's behaviour lives in a package for that harness alone.
   Each workaround names its entry in the harness data file ([ADR 9](0009-native-harness-features-first.md)), so it can be deleted when the harness closes the gap.
   That file and the code that reads it sit in neither harness's package, since they belong to no one harness.
7. **Works without mise.**
   A user needs neither mise nor Go to run the binary, and it never calls mise.

## Consequences

- Go commands find the module only in `cli/`.

## Options considered

- **Python:** users would need a runtime.
- **Rust:** its standard library has no JSON, HTTP, SHA-256 or archive support, so the commands planned here would start with many direct and transitive crates; it is also slower to build, and fewer contributors know it.
- **A module at the repository root:** takes in Go files that plugins ship.
- **A separate exit status for partial success:** every caller would handle one more code, and one that checks only for 1 would read a partial success as success; `--json` and stderr already say what was done and what was skipped.
- **Exit statuses as bit flags (1, 2, 4), combined when several apply:** callers would decode bits, and a script that checks for 1 would miss a combined status.
