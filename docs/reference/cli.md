# The `cech` CLI

The commands `cech` has now, and the output promises every command keeps ([ADR 15](../adr/0015-the-cech-cli.md)).
The golden files in `cli/internal/app/testdata/` hold the exact output of each command, and `commandline.golden` the help of every command.

## Commands

| Command | Does |
| --- | --- |
| `cech version` | Prints `cech <version>`; with `--json`, one object with `version`, `go`, `os` and `arch` |
| `cech build` | Writes the files generated from a marketplace's sources ([Generated files](generated-files.md)) |
| `cech completion <shell>` | Prints a completion script for the shell to source |
| `cech help [command]` | Prints the help of `cech` or of a command |

### `cech build`

| Flag | Effect |
| --- | --- |
| none | Writes the generated files, and prints `created <path>`, `updated <path>` or `removed <path>` for each file it changes, in path order |
| `--check` | Writes nothing, prints `create <path>`, `update <path>` or `remove <path>` for each file that differs from what it would write, and fails when any does, saying on stderr how many |
| `--dry-run` | Prints the same lines as `--check`, but does not fail |
| `--release V` | Builds a catalogue release ([A marketplace](marketplace.md#release-builds)) |
| `-C DIR` | Runs in `DIR`, the marketplace's root, instead of the current directory |
| `--json` | Prints one object with `mode`, `ok`, `files`, `skipped` and `errors`; each entry in `files` and `skipped` has a `path` and a `change` |

## Output

- Results go to stdout, and messages and errors to stderr.
- Every command except `help` and `completion` takes `--json`, and then prints one JSON object on stdout, even when it fails once its arguments and flag values are accepted.
  After an internal error, such as a panic, stdout holds no valid JSON, and the command exits 1.
- A command never reports success for a check it could not run, and after a partial success it says what it skipped.
- A command that writes files takes `--dry-run`, which reports the changes and writes nothing.

### Exit status

| Status | Means |
| --- | --- |
| 0 | Done |
| 1 | The command ran and met a failure, including a partial success |
| 2 | The command line is wrong, such as an unknown command or flag, a wrong number of arguments, flags that conflict, or a bad flag value |
| 3 | Something the command needs could not be reached, such as a file it is not allowed to read, so a retry may succeed |

When both 1 and 3 apply, the command exits 1, since a retry cannot fix the failure, and its output names both problems.
No other status is used.

### Compatibility

A minor release of the CLI may add fields to a JSON object.
Removing a field, renaming it or changing its meaning, and changing what an exit status means, is a breaking change ([ADR 24](../adr/0024-what-a-version-change-means.md)).
