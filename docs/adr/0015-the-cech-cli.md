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
   Every command that writes takes `--dry-run`, which reports the changes and writes nothing.
5. **Output is a contract.**
   - Results go to stdout, and messages and errors to stderr.
   - Every command except `help` takes `--json` and then prints one JSON object on stdout, even when it fails after parsing its arguments; after an internal error, such as a panic, stdout holds no valid JSON, and the command exits 1.
     A minor release may add fields; removing a field, renaming it or changing its meaning is a breaking change.
   - Exit status: 0 done; 1 the command ran and met a failure, including a partial success; 2 a wrong command line; 3 something it needs could not be reached, so it could not decide.
     When a command both meets a failure and cannot decide something else, it exits 1, since a retry cannot fix the failure, and its output names both.
     Other codes are not used.
   - A command never reports success for a check it could not run, and after a partial success it says what it skipped.
6. **Harness quirks stay in one place.**
   Code that knows one harness's behaviour lives in `internal/harness/claude` or `internal/harness/codex`.
   Each workaround names its entry in the harness data file ([ADR 9](0009-native-harness-features-first.md)), so it can be deleted when the harness closes the gap.
   That file, at `internal/harnessdata/harnesses.toml`, and the code that reads it live apart from both packages, since they belong to no one harness.
7. **Works without mise.**
   A user needs neither mise nor Go to run the binary, and it never calls mise.

## Consequences

- Only `cli/` has a `go.mod`, so Go commands run in `cli/`, or from the root with `-C cli`.

## Options considered

- **Python:** users would need a runtime.
- **Rust:** its standard library has no JSON, HTTP, SHA-256 or archive support, so the commands planned here would start with many direct and transitive crates; it is also slower to build, and fewer contributors know it.
- **A module at the repository root:** takes in Go files that plugins ship.
- **A separate exit status for partial success:** every caller would handle one more code, and one that checks only for 1 would read a partial success as success; `--json` and stderr already say what was done and what was skipped.
- **Exit statuses as bit flags (1, 2, 4), combined when several apply:** callers would decode bits, and a script that checks for 1 would miss a combined status.
