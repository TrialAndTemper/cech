# Evals

What a plugin's eval folder holds, what `cech eval --check` checks in it ([ADR 25](../adr/0025-evals.md)), and the run results, report and values of the release check.
The two harnesses are named `claude-code` and `codex` in files and messages.

## Layout

A plugin's eval folder is `evals/<category>/<plugin>/` at the marketplace root, beside the plugin folder `plugins/<category>/<plugin>/`.
It holds:

- `project/`, the project every run starts in, and optionally `project.staged/`, a change staged on top of it;
- `routing.md`, the skill's routing rows;
- one folder per case, named after the case.

`evals/` holds nothing else, apart from `evals/results/`, where `cech eval` writes the results of its runs, which stay out of git, and `evals/reports/`, where it writes its report files, which a pull request commits.
The check reports as a finding:

- anything else in `evals/`;
- a plugin category named `results` or `reports`, whose eval folders would fall in `evals/results/` or `evals/reports/`;
- in `evals/reports/`, anything but a file `<commit>.md` named by the full id of a commit, and a line of a report file that holds a local path, as [Recordings](#recordings) defines one;
- an eval folder for a plugin that does not exist;
- a folder where a file belongs, or a file where a folder belongs.

## The fixture project and the routing rows

- `project/AGENTS.md` has a section headed `## Routing` with a Markdown table whose first row is a header.
- No `AGENTS.md` in `project/` or in any `project.staged/` names the plugin's skill as `` `<skill>` ``, so an unrouted run sees no route to it.
- `project/` and every `project.staged/` hold no `CLAUDE.md`, `.claude/`, `.codex/` or `.agents/` at any depth, and no local path.
- `routing.md` holds only rows of a Markdown table, with no header and no blank line between them, each with as many cells as the `## Routing` table's header; at least one names the skill as `` `<skill>` ``.

## Cases

The format of a case is CECH's own ([ADR 25](../adr/0025-evals.md) item 2).
A case folder holds `prompt.md`, `cech.toml` and `recordings/`, and may hold a `project.staged/` that replaces the plugin's for this case; anything else is a finding.

- Case names are unique across the marketplace and are not `project` or `project.staged`.
- `prompt.md` is the prompt: the whole file, not empty, and not starting with a `---` line.
- Graders go in `cech.toml`, not in a `graders/` folder or a `case.yaml`.

### `cech.toml`

| Key | Families | Required | Value |
| --- | --- | --- | --- |
| `family` | all | yes | `content`, `control` or `trigger` |
| `expect` | all | yes | content `better`; control `same`; trigger `load` or `skip` |
| `wording` | trigger | yes | with `load`, `matched` or `oblique` (avoiding the description's words); with `skip`, `unrelated` or `near-miss` (close to the skill's subject but outside it) |
| `claim` | content | yes | the skill rule the case tests, one line |
| `graders` | content, control | yes | an array of tables, `[[graders]]`, at least one; see below |
| `recordings.<harness>.pass` | all | yes | names of recordings that must pass, at least one |
| `recordings.<harness>.fail` | all | yes | names of recordings that must fail, at least one |

`recordings` has a table for each harness, `claude-code` and `codex`, and a key outside its families is a finding.
Each plugin has at least one content case, one control case, three trigger cases that expect `load`, one of them `oblique`, and three that expect `skip`, one of them `near-miss`.

### Graders

A grader reads the reply or the files of a run, the same way on both harnesses.
A trigger case has no graders: its verdict comes from the load detectors below.

| Key | Types | Required | Value |
| --- | --- | --- | --- |
| `name` | all | yes | lowercase letters and digits, in words joined by single `-`, unique in the case; findings name the grader by it |
| `type` | all | yes | `regex` or `file_exists` |
| `pattern` | `regex` | yes | a regular expression in [Go's syntax](https://pkg.go.dev/regexp/syntax), such as `'(?m)^Refs: #\d+$'`, best written as a TOML literal string in single quotes |
| `match` | `regex` | no | `contains` (the default), `not_contains` or `count` |
| `count` | `regex` | with `match = "count"` | a whole number: the pattern matches exactly this many times |
| `target` | `regex` | no | `last_message` (the default), `files` (the paths the run created, one per line, sorted) or `file` |
| `path` | `regex` with `target = "file"` | yes | the relative path of a file in the run's workspace: slash-separated, with no `\`, no drive letter, and no empty, `.` or `..` part |
| `path` | `file_exists` | yes | a relative path, as for `regex`, that may hold `*` and `?`, which stay inside one folder, and `**`, which crosses folders; `[`, `]`, `{`, `}` and `!` are findings |
| `exists` | `file_exists` | no | boolean, default true: whether a created path matches the glob |

A key a grader's type does not take is a finding, as is a `pattern` that names `/cech/`, the root of the placeholders a recording writes for local paths, which a live run does not hold.

## Recordings

A recording is a folder `recordings/<harness>/<name>/`, holding:

- `trace.jsonl`, the run's output trimmed to what graders and the load detectors read;
- optionally `workspace/`, the run's files after it, and `created.txt`, the paths the run created, one per line; without `created.txt`, every file in `workspace/` counts as created, and an empty `created.txt` records a run that created none.

Every recording folder is named in its harness's `pass` or `fail` list in `cech.toml`, every name there is a folder, and no name is in both.

A trace holds one JSON object per line, read strictly, so a duplicate key, invalid UTF-8 or a lone surrogate escape makes the line invalid:

- for `claude-code`, from `claude -p --output-format stream-json`: `assistant` lines with `text` and `tool_use` blocks, `user` lines with `tool_result` blocks keeping `tool_use_id` and `is_error`, and a last `result` line with `subtype`, `is_error` and `result`;
- for `codex`, from `codex exec --json`: `item.completed` lines with an `agent_message` or a `command_execution` (its `command`, `aggregated_output`, `exit_code` and `status`), and a last `turn.completed` line with only `type`.

Any other line, key or block is a finding, as is a trace without its last line.
In a recording, the run's workspace is written as `/cech/workspace`, the plugin's folder as `/cech/plugin` and the harness's home as `/cech/home`.
With each placeholder removed, together with the path parts that follow it, no string in `trace.jsonl` and no path in `created.txt` holds a local path: one under `/home/`, `/Users/`, `/tmp/`, `/private/` or `/var/folders/`, or a drive letter followed by `:\` at the start of a word.
A path part ends at a character other than a letter, digit or one of `._@+~-`, or at a `..` part.
The Markdown and link checks leave recordings out, since a model wrote them, and the Markdown checks leave out `routing.md`; the term check covers every file.

## Grading a recording

A content or control recording is graded by the case's graders.
The reply is the `result` line's `result` on `claude-code` and the last `agent_message` on `codex`; a `file` target is `workspace/<path>`, and the grader fails without it.
A recording marked pass is a finding unless every grader passes, and one marked fail is a finding unless at least one fails.
When a grader in `cech.toml` is invalid, the case's recordings are read but not graded, so the graders left cannot give a verdict the case does not declare.

A trigger recording is graded by the load detector of its harness:

- on `claude-code`, the skill loads when a `Skill` `tool_use` naming `<skill>` or `<plugin>:<skill>` has a `tool_result` that is not an error;
- on `codex`, it loads when one `command_execution`'s output has a line ending in the `name` line of `SKILL.md`, from the `name` key on, and one ending in the first line of its `description`.
  Both are found as `cech build` reads `SKILL.md`, and matched as `SKILL.md` writes them, quotes included, after trailing whitespace is removed, whatever the command's exit status.
  A line may start with a prefix that does not end in a letter or digit, such as the line number `grep -n` adds.

A recording marked pass loads the skill for `expect = "load"` and does not for `expect = "skip"`; one marked fail, the other way round.
A finding about a verdict names the recording and the graders, or the detector, that gave the other verdict.

## Run results and reports

The release check ([ADR 27](../adr/0027-the-release-check.md), the [`cech eval` command](cli.md#the-release-check)) writes each run to `evals/results/<commit>/<harness>/<case>/<arm>/<n>/`, where `<n>` counts the arm's runs from 1; a run that errors is made once more, in `<n>-rerun/`, and the rerun takes its place.
The arms are `with` and `without` for content and control cases, with `latest-release` too for a content case of a changed plugin that the latest final release had, and `routed` and `unrouted` for trigger cases.
A run's folder holds a recording, as [Recordings](#recordings) defines it, and:

- `loaded.json`, the plugins, skills and MCP servers the harness loaded;
- `run.json`, the run's time in seconds, harness, image ID, model and kind of credential, and why it errored, if it did;
- `telemetry.json`, only these from the run's telemetry: the tokens, the cost when the harness gives one, the model requests, the highest retry attempt, the skills and plugins seen, and the whole input of the first request of the main loop;
- `key.json`, the image ID, model, `cech` version, kind of credential and harness version the run was made with, written once it finished; a later `cech eval` on the same commit with the same key reuses the run, or its rerun, instead of making it again.

An interrupt removes the folder of each run it stopped.

The report file `evals/reports/<commit>.md` comes from a template the CLI holds, so it has the same form at every release.
It names the CLI's version, the commit, and the image with its ID, and for each harness its version, model, kind of credential, runs made or reused, the tokens of its finished runs and, on Claude Code, their cost, and why the check stopped early if it did.
It then has these sections for each harness:

- Bars: each bar's result, its range and the runs it read, then each control or drop bar whose range shows a difference smaller than the threshold;
- Arms: each arm's runs, passes, errors and pass rate range;
- Graders: each grader's passes in each arm of a content or control case;
- What each skill version costs: its description size and the input tokens it adds to the first request;
- Why runs errored: each reason and how many runs it holds;
- Cases without a result: those whose runs errored too often.

A range is the span of pass rates, or of differences between two arms, that the runs allow at the false-alarm rate.
It gives no path from the machine it ran on.

The CLI holds these values for every marketplace:

| Value | Is |
| --- | --- |
| Image | The image `CECH_EVAL_IMAGE` names, or else the digest the eval image workflow published, which a pull request puts in the CLI; until then the CLI holds none |
| Models | `claude-haiku-5-5` on Claude Code and `gpt-6-luna` on Codex |
| Hosts | The only hosts a run may reach, through the proxy: `api.anthropic.com`, `statsig.anthropic.com` and `console.anthropic.com` for Claude Code; `chatgpt.com`, `api.openai.com` and `auth.openai.com` for Codex |
| Run timeout | 10 minutes |
| Error share | A case has no result when more than 10% of its runs still error after their rerun, checked from its 20th run on and at the end |
| Runs at once | 6 on Claude Code and 4 on Codex |
| Bars | `content passes with the plugin`: the `with` arm passes in more than 80% of runs; `routed skill loads when it should`: in more than 70% of the `routed` runs of `load` cases; `routed skill seldom loads otherwise`: in less than 10% of the `routed` runs of `skip` cases; `control does not differ`: `with` and `without` within the threshold; `no drop since the latest final release`: `with` no more than the threshold below `latest-release` |
| Threshold | 10 points, for the control bar and for a drop since the latest final release |
| False-alarm rate | 1 in 20 per bar |
| Cap | 500 runs per arm per case, enough to miss a 15-point drop 1 in 5 times; the estimates of `--dry-run` read it |
| Budget | 10,000 runs per harness per release |
| Median run time | 10 s on Claude Code and 20 s on Codex, for the estimates of `--dry-run` |

A drop near the threshold usually ends with no result; a person then decides whether to release ([ADR 27](../adr/0027-the-release-check.md) item 5).
