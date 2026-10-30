# Evals

What a plugin's eval folder holds, and what `cech eval --check` checks in it ([ADR 25](../adr/0025-evals.md)).
The two harnesses are named `claude-code` and `codex` in files and messages.

## Layout

A plugin's eval folder is `evals/<category>/<plugin>/` at the marketplace root, beside the plugin folder `plugins/<category>/<plugin>/`.
It holds:

- `project/`, the project every run starts in, and optionally `project.staged/`, a change staged on top of it;
- `routing.md`, the skill's routing rows;
- one folder per case, named after the case.

`evals/` holds nothing else, apart from `evals/results/`, where `claude plugin eval` writes its results, which stay out of git.
The check reports as a finding:

- anything else in `evals/`;
- a `mocks/` folder at any depth;
- a plugin category named `results`, whose eval folders would fall in the git-ignored `evals/results/`;
- an eval folder for a plugin that does not exist;
- a `prompt.md` or `case.yaml` anywhere but at the top of a case folder, which `claude plugin eval` would load as a case.

## The fixture project and the routing rows

- `project/AGENTS.md` has a section headed `## Routing` with a Markdown table whose first row is a header.
- No `AGENTS.md` in `project/` or in any `project.staged/` names the plugin's skill as `` `<skill>` ``, so an unrouted run sees no route to it.
- `project/` and every `project.staged/` hold no `CLAUDE.md`, `.claude/`, `.codex/` or `.agents/` at any depth, and no local path.
- `routing.md` holds only rows of a Markdown table, with no header and no blank line between them, each with as many cells as the `## Routing` table's header; at least one names the skill as `` `<skill>` ``.

## Cases

A case folder holds `prompt.md` in the format of `claude plugin eval`, `graders/` with at least one `<name>.md`, `cech.toml` and `recordings/`.
It may hold `case.yaml`, the files it and the graders name, and a `project.staged/` that replaces the plugin's for this case.

- Case names are unique across the marketplace and are not `project` or `project.staged`.
- `prompt.md` has no `name`, and its `plugins` is exactly `["../../../../plugins/<category>/<plugin>"]`.
- `case.yaml` has only `schema_version`, `name` (the folder's name), `description`, `tags` and `context`; a content or control case has no `context.history_file`.

### `cech.toml`

| Key | Families | Required | Value |
| --- | --- | --- | --- |
| `family` | all | yes | `content`, `control` or `trigger` |
| `expect` | all | yes | content `better`; control `same`; trigger `load` or `skip` |
| `wording` | trigger | yes | with `load`, `matched` or `oblique` (avoiding the description's words); with `skip`, `unrelated` or `near-miss` (close to the skill's subject but outside it) |
| `critical` | content | no | boolean, default false; read by the runner, not by `cech eval --check` |
| `claim` | content | yes | the skill rule the case tests, one line |
| `recordings.<harness>.pass` | all | yes | names of recordings that must pass, at least one |
| `recordings.<harness>.fail` | all | yes | names of recordings that must fail, at least one |

`recordings` has a table for each harness, `claude-code` and `codex`, and a key outside its families is a finding.
Each plugin has at least one content case, one control case, three trigger cases that expect `load`, one of them `oblique`, and three that expect `skip`, one of them `near-miss`.

### Graders

Each grader file starts with YAML frontmatter whose `type` is one of `claude plugin eval`'s: `regex`, `tool_used`, `tool_order`, `file_exists`, `llm` or `baseline`.
It takes the keys its type takes in `claude plugin eval`, plus `weight`, a positive number, and `arm`, `with-only` (graded only in the arm with the plugin) or `both`, which the check does not read; any other key or value is a finding:

- `match` is `contains` (the default), `not_contains` or `count:N`, with `N` a whole number; with `count:N`, the pattern must not match an empty string;
- `target` is `last_message` (the default), `files`, or `{source: file, path: <relative path>}`; `trace` and `mock_calls` are findings;
- `flags` holds only `i` and `s`, each at most once;
- `min` and `max` are whole numbers, `min` at most `max`, and `exists` is a boolean;
- the `path` of a `file_exists` grader is a relative glob of `*`, `?` and `**`, with no empty, `.` or `..` part.

Where the graders apply:

- `regex` and `file_exists` graders apply on both harnesses; `tool_used` and `tool_order` name Claude Code tools and apply on `claude-code` only; `llm` and `baseline` graders call a model and are not graded offline.
- Each content and control case has a `regex` or `file_exists` grader.
- A trigger case has exactly one grader, so `claude plugin eval` can run it: `tool_used` with `tool: Skill`, an `input_match` that matches the skill as `<skill>` or `<plugin>:<skill>` and no other name, `min` at least 1 for `expect = "load"`, and `min` and `max` 0 for `expect = "skip"`; its verdict comes from the load detectors below.
- Every `pattern` and `input_match` keeps to the subset of JavaScript regular expressions that Go reads the same way: literals, `^`, `$`, `.`, the common escapes and classes, bracket classes, groups, alternation and the usual quantifiers, without the `u` flag.
  Lookaround, back-references, named groups, inline flags, `\p`, `\u` and `\x` escapes, and a pattern holding `/cech/` are findings.

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
With those removed, no string in a recording or another `.jsonl` file in a case holds a local path such as one under `/home/`, `/Users/` or `/tmp/`.

## Grading a recording

A content or control recording is graded by the case's deterministic graders for its harness, as `claude plugin eval` grades a run.
The reply is the `result` line's `result` on `claude-code` and the last `agent_message` on `codex`; a `files` target is the created paths, one per line, sorted; a `{source: file, path: P}` target is `workspace/P`, and the grader fails without it.
A recording marked pass is a finding unless every such grader passes, and one marked fail is a finding unless one fails.

A trigger recording is graded by the load detector of its harness:

- on `claude-code`, the skill loads when a `Skill` `tool_use` naming `<skill>` or `<plugin>:<skill>` has a `tool_result` that is not an error;
- on `codex`, it loads when one `command_execution`'s output has a line ending, after trailing spaces, with `name: <skill>` and one ending with the first line of the skill's `description`, whatever the command's exit status; a line may start with a prefix that does not end in a letter or digit, such as the line number `grep -n` adds.

A recording marked pass loads the skill for `expect = "load"` and does not for `expect = "skip"`; one marked fail, the other way round.
A finding about a verdict names the recording and the graders, or the detector, that gave the other verdict.
