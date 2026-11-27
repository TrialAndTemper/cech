# Evals

What a plugin's eval folder holds, and what `cech eval --check` checks in it ([ADR 25](../adr/0025-evals.md)).
The two harnesses are named `claude-code` and `codex` in files and messages.

## Layout

A plugin's eval folder is `evals/<category>/<plugin>/` at the marketplace root, beside the plugin folder `plugins/<category>/<plugin>/`.
It holds:

- `project/`, the project every run starts in, and optionally `project.staged/`, a change staged on top of it;
- `routing.md`, the skill's routing rows;
- one folder per case, named after the case.

`evals/` holds nothing else, apart from `evals/results/`, where `cech eval` will write the results of its runs, which stay out of git.
The check reports as a finding:

- anything else in `evals/`;
- a plugin category named `results`, whose eval folders would fall in the git-ignored `evals/results/`;
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
