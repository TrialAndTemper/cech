# The `cech` CLI

The commands `cech` has now, and the output promises every command keeps ([ADR 15](../adr/0015-the-cech-cli.md)).
The golden files in `cli/internal/app/testdata/` hold the exact output of each command, and `commandline.golden` the help of every command.

## Commands

| Command | Does |
| --- | --- |
| `cech version` | Prints `cech <version>`; with `--json`, one object with `version`, `go`, `os` and `arch` |
| `cech build` | Writes the files generated from a marketplace's sources ([Generated files](generated-files.md)) |
| `cech eval` | Runs the release check on a marketplace's eval cases ([ADR 27](../adr/0027-the-release-check.md)); with `--check`, checks the cases in `evals/` without running an agent ([Evals](evals.md)) |
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

### `cech eval`

Without `--check`, `cech eval` runs the release check: the eval cases of each plugin changed since the latest final release, on each harness, each run in a fresh container, adding runs until each bar passes, fails or has no result.
With `--check`, it checks the eval cases and their recordings without running an agent.

| Flag | Effect |
| --- | --- |
| none | Runs the release check; prints the plan line of each harness, as `--dry-run` does, then the result of each bar as `<harness>: <plugin> <case>: <bar>: <result>`, with `trigger cases` as the case of a bar over a plugin's trigger cases, and then `report: <path>` for the report file it wrote; fails when a bar fails or has no result |
| `--plugin NAME,…` | Checks these plugins instead of those changed since the latest final release, each once however often it is named; a name that is no plugin of the marketplace is a wrong command line |
| `--harness H` | Runs on `claude-code` or `codex` alone; the other harness is left out of the report, not reported with no result, so a report file to commit comes from a run on both |
| `--parallel N` | Makes `N` runs of a harness at once; 0, the default, means 6 on Claude Code and 4 on Codex |
| `--dry-run` | Prints only the plan line of each harness, `<harness>: <n> runs to start, at most <m>; about <a> to <b> minutes`, where `<n>` is the first five runs of each arm and `<m>` the most it may make, up to the cap on each arm a bar reads and within the budget; it runs and writes nothing, and counts no run it would reuse |
| `--check` | Checks the eval folders, and prints each finding on stdout as `<path>: <problem>`, with the path from the marketplace root, in path order; with any finding, it fails and says on stderr how many; it takes no `--plugin`, `--harness`, `--parallel` or `--dry-run` |
| `-C DIR` | Runs in `DIR`, the marketplace's root, instead of the current directory; an empty `DIR` is a wrong command line |
| `--json` with `--check` | Prints one object with `ok`, `cases` (the number of case folders found), `recordings` (the number of recordings read), `findings`, each with a `path` and a `message`, and `errors` |
| `--json` | Prints one object with `ok`, `commit`, `plugins`, `dry_run`, `plan` (per harness: `harness`, `first_runs`, `most_runs`, `first_minutes`, `most_minutes`), `report` when it wrote one, `bars` (each with `harness`, `plugin`, `case`, `bar`, `result` and `runs`, the run count of each arm the bar reads) and `errors` |

A bar is one of `content passes with the plugin`, `routed skill loads when it should`, `routed skill seldom loads otherwise`, `control does not differ` and `no drop since the latest final release` ([Evals](evals.md#run-results-and-reports) gives their values).
A result is `pass`, `fail`, or `no result`: for a bar still unclear at the cap or when the budget runs out, for a case whose runs errored too often, or for every bar of a harness that has no result.

#### The release check

The latest final release is the tag `vX.Y.Z` with the highest version and no `-rc.N` ([Release builds](marketplace.md#release-builds)); with no such tag, every plugin counts as new.
A plugin has changed when a file in its folder differs between `HEAD` and the commit of `main` the release was cut from, so the versions the release itself writes do not count; a change that `cech build` writes into every plugin folder, such as one from `data/marketplace.toml` or `LICENSE`, changes every plugin.
A plugin has also changed when a plugin it depends on is new or changed.
With no plugin changed and no `--plugin`, it says so and exits 0.

It needs a git checkout, or it exits 3.
It refuses, and exits 1, when `plugins/`, `evals/` (apart from `evals/reports/` and `evals/results/`), `.claude-plugin/`, `data/marketplace.toml` or `LICENSE` hold a change that is not committed, untracked files included, since the report names the commit it ran on; commit the change and run it again.
It also refuses, and exits 1, when `cech eval --check` would find a problem in the eval cases, or when a plugin depends on one the marketplace does not hold.
When checking the cases is only denied a read, it exits 3.
In a shallow clone without the release tags, it exits 3 and says to fetch them.
It installs each plugin with those it depends on, and copies the fixture projects, from `HEAD`'s tree, so nothing uncommitted or ignored reaches a run.
It reads no variable that marks CI, such as `CI` or `GITHUB_ACTIONS`, so it behaves the same on a maintainer's machine and in CI.

It needs Podman or Docker, and prefers Podman; with neither, it exits 3.
Each run reaches only its harness's hosts, through a proxy on an internal network; this was checked on Podman, where a run could resolve no other name, but not on Docker.
It runs each harness in the image `CECH_EVAL_IMAGE` names, or else in the image whose digest the CLI holds ([Evals](evals.md#run-results-and-reports)); with neither, it exits 3.
The CLI holds no digest until the eval image workflow has published one, so until then build the image and name it, as on Podman: `podman build --tag cech-eval:local images/eval/`, then `CECH_EVAL_IMAGE=localhost/cech-eval:local cech eval`.
It pulls the image if the engine does not hold it, and exits 3 if it cannot, then runs everything by the image's ID, so an image rebuilt under the same name is a new image to it.
Claude Code is not in the image: before the first Claude Code run, `cech eval` installs the version the image pins into `cech/claude-code/` in the user's cache folder (`~/.cache` on Linux, `~/Library/Caches` on macOS), and each run mounts it read-only.

Each harness needs one credential.
For each variable in the table, in order, `cech eval` looks in the environment and then in the system keyring, under the service `cech` with the variable's name as the user name; the first one found wins.

| Harness | Credential, first found wins |
| --- | --- |
| Claude Code | `ANTHROPIC_API_KEY`; else `CLAUDE_CODE_OAUTH_TOKEN`, from `claude setup-token` |
| Codex | `CODEX_API_KEY`; else the ChatGPT login `auth.json` in `CODEX_HOME`, or `~/.codex`, never in the keyring; before the first run, `cech eval` runs `codex login status` on a copy, which is meant to refresh it, and puts the copy back if Codex changed it |

The keyring is the Secret Service on Linux, the Keychain on macOS and the Credential Manager on Windows; on Linux, `secret-tool store --label="cech: CLAUDE_CODE_OAUTH_TOKEN" service cech username CLAUDE_CODE_OAUTH_TOKEN` stores the token.
An API key or token is passed to a container as an environment variable, never on a command line or in a file of the run; a ChatGPT login is copied into the run's home as `auth.json`.

It writes each run's results under `evals/results/<commit>/` and the report to `evals/reports/<commit>.md` ([Evals](evals.md#run-results-and-reports)).
A finished run with the same key is reused, not run again ([Evals](evals.md#run-results-and-reports)), so a check that stopped goes on where it stopped.
Only one `cech eval` at a time may run on a commit's results; another exits 3.

A harness has no result when it has no credential, when installing Claude Code, refreshing the Codex login, starting its proxy or reading its version fails, when not one of its runs gives a result, or when the runner itself cannot work, such as a run folder it cannot write.
Its exit status is then:

| Status | When |
| --- | --- |
| 0 | Every bar passed, no plugin changed, or a `--dry-run` |
| 1 | A bar failed or has no result, a refusal, a harness with no credential, a runner that cannot work, or an interrupt |
| 3 | A harness with no result because installing Claude Code, refreshing the Codex login, starting its proxy or reading its version failed, or not one of its runs gave a result, while every other harness passed; or no engine, no image, an image it cannot pull, no git checkout, a denied read, a shallow clone, a results folder or report file it cannot create, or another `cech eval` on the same commit |

When both 1 and 3 apply, it exits 1.

Every container it starts has no capabilities and a limit on processes and memory, and carries the label `cech-eval`, whose value names its batch: the proxy, forwarder and runs of one harness in one `cech eval`.
An interrupt stops the runs, removes the batch's containers and networks and the folder of each run it stopped, keeps the finished runs, writes no report and exits 1.
A second interrupt kills `cech eval` at once by its signal, with no exit status of its own.
It may leave behind the batch's containers and networks, and the telemetry receiver's folder `cech/otlp-*` in the user's cache folder; `podman rm --force $(podman ps --all --quiet --filter label=cech-eval)` and `podman network rm $(podman network ls --quiet --filter name=cech-eval-)` remove the containers and networks, and the same commands work with `docker`.

#### Checking the cases

With `--check`, it reads the marketplace as `cech build` does.
When `cech build` would reject a source, it reports that as an error and checks no eval folder, so a folder with no `data/marketplace.toml` fails.
A marketplace with no plugins and no `evals/` passes with nothing checked, and says so on stderr, or with `--json` reports no cases.
A denied read skips the rest of its case or folder.
It also skips the check of minimum cases for the plugin it affects, or for every plugin when the denied folder is above the case folders, such as `evals/` or a category.
An error says what was skipped.
The command then exits 3, unless a finding or another error makes it exit 1.

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
No other status is used, apart from a second interrupt, which kills `cech eval` by its signal.

### Compatibility

A minor release of the CLI may add fields to a JSON object.
Removing a field, renaming it or changing its meaning, and changing what an exit status means, is a breaking change ([ADR 24](../adr/0024-what-a-version-change-means.md)).
