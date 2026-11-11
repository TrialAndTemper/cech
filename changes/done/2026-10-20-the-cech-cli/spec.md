# cech CLI: version and internal errors

Implemented 2026-10-29; not maintained: the code and its tests are current.

How the `cech` CLI finds the version it reports, and how it reports an internal error, under ADR 15 item 5 and ADR 16 item 2.

## Rules

1. `cech version` reports the version that a release build stamps with `-ldflags -X` into `internal/version.stamped`; else the main module's version from the build information, as `go install` of a version records it; else `0.0.0-dev`.
   A leading `v` is dropped.
2. An internal error, such as a panic in a command's code, exits 1 and reports the error, with a stack trace, on stderr.

## Notes

- After an internal error, stdout holds no valid result, even with `--json`.
- Go stamps a version from git tags only for a module at the repository root, so a `go build` of `cli/` records `(devel)` (checked on 2026-10-27 with Go 1.27.3); a release build therefore needs the stamp.
- Two exit statuses are outside `cech`'s control: the one a shell reports when a signal kills it, and the 2 that Go itself exits with after a fatal runtime error or a SIGQUIT, which does not mean a wrong command line.
