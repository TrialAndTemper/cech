# Evals and the eval gate

What a plugin's eval folder holds, what `cech eval --check` checks, and how the eval gate runs it.
The ADRs behind it are ADR 5 item 5, ADR 13 item 4, ADR 15 item 5, ADR 16 item 1 and ADR 25.
It builds on the change that added `cech build` (ADR 18 items 3 and 7), which reads the plugins under `plugins/`; that change also added the YAML module.
The runner, the release check and the script grader that ADR 25 item 4 allows are not part of this change; [brief.md](brief.md) holds notes for them.
The two harnesses are named `claude-code` and `codex` in files and messages.

## Layout

1. A plugin's eval folder is `evals/<category>/<plugin>/` at the marketplace root, where `plugins/<category>/<plugin>/` is a plugin folder as `cech build` reads it.
   It holds `project/`, optionally `project.staged/`, `routing.md`, and one folder per case.
2. `evals/` holds nothing else, apart from `evals/results/`, where `cech eval` will write the results of its runs, which `.gitignore` lists.
   Anything else in `evals/` is a finding.
   So is a category named `results`.
   An eval folder for a plugin that does not exist is a finding.

## The fixture project

3. `project/` holds the project every run starts in, and the optional `project.staged/` a change staged on top of it; [brief.md](brief.md) says how the runner builds them.
4. `project/AGENTS.md` exists and holds a section headed `## Routing` with a Markdown table whose first row is a header.
   No `AGENTS.md` in `project/` or in any `project.staged/`, the plugin's or a case's, names the plugin's skill as `<skill>` in backquotes, so an unrouted run sees no route to it.
   `project/` and every `project.staged/` hold no `CLAUDE.md`, `.claude/`, `.codex/` or `.agents/` at any depth: Claude Code reads no `AGENTS.md` where a `CLAUDE.md` is present, and the others would add settings or skills of their own.
5. Files in `project/` and every `project.staged/` hold no local path, as rule 22 defines it.

## The routing row

6. `routing.md` holds one or more rows of a Markdown table, and nothing else: no header and no blank line between rows.
   Each row has as many cells as the header of the table under `## Routing` in `project/AGENTS.md`, with cells split at each `|` not escaped as `\|`.
   At least one row names the skill, as `<skill>` in backquotes.

## Cases

The format of a case is CECH's own (ADR 25 item 2).

7. A case folder holds `prompt.md`, `cech.toml` and `recordings/`, and may hold `project.staged/`, which replaces the plugin's for this case; anything else is a finding.
8. A case's name is its folder's name, other than `project` and `project.staged`.
   Case names are unique across the marketplace, since run results name cases by name.
9. `prompt.md` is the prompt: the whole file, sent as it is.
   It is not empty, and does not start with a `---` line, which would mark frontmatter.

## Graders

10. A content or control case's graders are the tables of `graders` in `cech.toml`, `[[graders]]`, with these keys:

    | Key | Types | Required | Value |
    |---|---|---|---|
    | `name` | all | yes | lowercase letters and digits, in words joined by single `-` |
    | `type` | all | yes | `regex` or `file_exists` |
    | `weight` | all | no | a finite positive number |
    | `arm` | all | no | `with-only` or `both` |
    | `pattern` | `regex` | yes | a regular expression |
    | `match` | `regex` | no | `contains` (the default), `not_contains` or `count` |
    | `count` | `regex` | with `match = "count"` only | a whole number |
    | `target` | `regex` | no | `last_message` (the default), `files` or `file` |
    | `path` | `regex` | with `target = "file"` only | a relative path |
    | `path` | `file_exists` | yes | a relative glob |
    | `exists` | `file_exists` | no | a boolean, default true |

11. A relative path is slash-separated, with no `\`, no drive letter, and no empty, `.` or `..` part.
    A glob is a relative path that may hold `*` and `?`, which stay inside one folder, and `**`, which crosses folders; `[`, `]`, `{`, `}` and `!` are findings.
12. A grader reads the reply or the files of a run, and grades both harnesses alike.
    Its `name` is unique in the case, and findings name the grader by it, or by `#<n>`, its place among the case's graders, when it has no valid name.
13. Each content and control case has at least one grader.
    A trigger case has none: its verdict is the load the runner detects (rule 24).

## Regular expressions

14. A `pattern` is a regular expression in Go's syntax, which `cech` compiles; inline flags such as `(?i)`, `(?s)` and `(?m)` set its flags.
15. A pattern whose literal characters, with escapes removed, include `/cech/`, the root of rule 22's placeholders, is a finding, since a live run has real paths there.

## Keys

16. A key a grader's type does not take, or a key no rule names, is a finding.
17. A `graders/` folder or a `case.yaml` in a case folder is a finding, since graders go in `cech.toml`.

## cech.toml

18. `cech.toml` has these keys, and no others:

    | Key | Families | Required | Value |
    |---|---|---|---|
    | `family` | all | yes | `content`, `control` or `trigger` |
    | `expect` | all | yes | content `better`; control `same`; trigger `load` or `skip` |
    | `wording` | trigger | yes | with `expect = "load"`, `matched` or `oblique`; with `skip`, `unrelated` or `near-miss` |
    | `critical` | content | no | boolean, default false |
    | `claim` | content | yes | the skill rule the case tests, one line, not empty |
    | `graders` | content, control | yes | the graders of rule 10 |
    | `recordings.<harness>.pass` | all | yes | names of recordings that must pass, at least one |
    | `recordings.<harness>.fail` | all | yes | names of recordings that must fail, at least one |

    `<harness>` is `claude-code` or `codex`, and both are present; a skill limited to one harness (ADR 10 item 4) waits for the change that records that limit in the plugin.
    A key outside its families is a finding.
    `oblique` means the prompt avoids the words of the skill's description; `near-miss` means it is close to the skill's subject but outside it.
    `cech` checks the labels and the counts of rule 19, not the wording itself.
19. Each plugin has, in its eval folder, at least one content case, one control case, three trigger cases with `expect = "load"`, one of them `oblique`, and three with `expect = "skip"`, one of them `near-miss`.

## Recordings

20. A recording is a folder `recordings/<harness>/<name>/`, with:
    - `trace.jsonl`, trimmed as rule 21 says for its harness;
    - optionally `workspace/`, the files in the run's workspace after the run, at their relative paths;
    - optionally `created.txt`, the paths in `workspace/` that the run created, one per line; without it, every file in `workspace/` counts as created, and an empty one records a run that created none.

    Every recording folder is named in its harness's `pass` or `fail` list, and every name there is a folder; no name is in both.
21. `trace.jsonl` holds one JSON object per line, trimmed to what graders and the runner read; any other line, key or block, or a line that is not a JSON object, is a finding:
    - for `claude-code`, from `claude -p --output-format stream-json`: `assistant` lines with only `type` and `message`, a `message` with only `role` and `content`, and blocks `text` (`type`, `text`) or `tool_use` (`type`, `id`, `name`, `input`); `user` lines with only `type` and `message`, holding only `tool_result` blocks with only `type`, `tool_use_id` and `is_error`, where a missing `is_error` means false, since the harness leaves the key out of a result that is not an error; and a last line of `type` `result` with only `type`, `subtype`, `is_error` and `result`;
    - for `codex`, from `codex exec --json`: `item.completed` lines with only `type` and `item`, an `item` of type `agent_message` (`type`, `text`) or `command_execution` (`type`, `command`, `aggregated_output`, `exit_code`, `status`), and a last line of `type` `turn.completed` with only `type`.

    The last line is required: a trace without it, or with it anywhere else, is a finding.
    A line is read strictly: a name repeated in one object, invalid UTF-8 or a lone surrogate escape such as `\ud800` makes it no JSON object.
    Thinking, reasoning, usage and every other line are dropped.
    Each `input` keeps the bytes the run wrote, apart from rule 22.
22. In a recording, the run's workspace is written `/cech/workspace`, the plugin's folder `/cech/plugin`, and the harness's home `/cech/home`; no real system has a `/cech` root.
    With each placeholder removed together with the parts of the path it starts, which end at a character other than a letter, digit or one of `._@+~-`, or at a `..` part, no string in `trace.jsonl`, and no path in `created.txt`, holds a local path: one containing `/Users/`, `/tmp/`, `/private/`, `/var/folders/`, `/home/`, or a drive letter followed by `:\` that no letter, digit or `_` comes before.
    Paths under `workspace/` are read relative to it, so they hold none.

## Grading a recording

23. `cech eval --check` grades each content and control recording with the case's graders:
    - `last_message` is the `result` of the `result` line on `claude-code`, and the `text` of the last `agent_message` on `codex`;
    - `files` is the paths of the created files, one per line, sorted;
    - `file` is the content of `workspace/<path>`, and the grader fails when it is missing;
    - `regex` passes when the pattern matches the target; with `not_contains`, when it does not; with `count`, when it matches exactly `count` times without overlap;
    - `file_exists` passes when a created file matches `path`, or, with `exists = false`, none does.

    `weight` and `arm` play no part.
    A recording marked pass is a finding unless every grader passes on it, and one marked fail is a finding unless at least one fails.
    A case with a grader that breaks rule 10, 11, 14, 15 or 16 has its recordings read but not graded, since the graders left could give a verdict the case does not declare.
24. A trigger case's recording is graded by the runner's load detector instead:
    - on `claude-code`, the skill loads when a `tool_use` named `Skill`, whose `input` `skill` is `<skill>` or `<plugin>:<skill>`, has a `tool_result` that is not an error;
    - on `codex`, the skill loads when the `aggregated_output` of one `command_execution` has a line ending with the `name` line of `SKILL.md`'s frontmatter, from the `name` key on, and a line ending with the first line of the `description` value (for a block scalar, its first content line).
      Both are found as `cech build` reads `SKILL.md`, and each is matched as `SKILL.md` writes it, quotes included, after trailing whitespace is removed from each line.
      A line may start with a prefix that does not end in a letter or digit, such as the line number `grep -n` adds, so `rename: <skill>` is not the name line.
      The command's exit status plays no part, since a command can read the file and then fail.

    A recording marked pass of an `expect = "load"` case loads the skill, and one marked fail does not; for `expect = "skip"`, the other way round.
25. A finding about a verdict names the recording and the graders, or the detector, that gave the other verdict.

## Command

26. `cech eval --check` works on the marketplace in the current folder, or in the folder that `-C` names.
    `cech eval` without `--check` is a wrong command line, exit status 2, until the runner exists.
    So is an empty `-C`, as for `cech build`.
    A folder where a file belongs, or a file where a folder belongs, is a finding.
27. It reads the marketplace as `cech build` does.
    A source `cech build` rejects is an error, reported as `cech build` reports it, and the check of the eval folders is skipped; it says so on stderr and in `errors`.
    So a folder with no `data/marketplace.toml` fails, as for `cech build`.
    A marketplace with no plugins and no `evals/` passes with nothing checked, and says so on stderr, or with `--json` reports no cases.
28. Its findings are its result, so it prints them on stdout, one per line as `<path>: <problem>`, in path order, with the path from the marketplace root.
    It names every finding, not only the first per file.
    With any finding, it says on stderr how many there are.
    It prints each error on stderr.
29. With `--json`, it prints one object with `ok`, a boolean; `cases` and `recordings`, the numbers of each it found; `findings`, a list in which each has a `path` and a `message`; and `errors`, a list of the errors, each a message.
    It then prints nothing on stderr.
30. The exit status is 0 with no findings, 1 with any or with an error other than a denied read, 2 for a wrong command line, and 3 when it cannot read the root or is denied a read and nothing else failed.
    A denied read skips the rest of its case or folder, and the check goes on with the rest.
    It also skips rule 19's minimums, which it can no longer judge, for that plugin, or for every plugin when a folder above the cases is denied.

## Gate and generated files

31. `mise run lint:evals` is part of `mise run lint`, and so of `mise run check`; `.gitignore` lists `evals/results/` at any depth.
    It builds `cech` from the checkout and runs `cech eval --check` on the repository, never a `cech` on `PATH`.
32. The Markdown checks and the link check leave out `evals/**/recordings/`, since the files there are what a model wrote, and the Markdown checks leave out `evals/**/routing.md`, a table without its header.
    The term check covers every file.
33. `cech build` writes, for each plugin in the code owners file, a line for `/evals/<category>/<plugin>/` that names the same people as its plugin folder's line, right after that line.
34. A test fixture marketplace under `cli/` testdata has a complete plugin: a fixture project with a routing table and a staged change, a `routing.md`, one content case, one control, and the trigger cases of rule 19 with each `wording`, each with a pass and a fail recording per harness, among them trimmed real runs of both harnesses.
    Tests break it one rule at a time, and include a grader that always passes, which its fail recording catches, and a recording of each harness that each detector must read correctly.
35. The harness data file records, as behaviours CECH relies on, the trace format of `codex exec --json` and the load detection of rule 24 on each harness.
    It records, as gaps with what CECH does instead: `claude plugin eval` grades no recorded run, loads no project instructions and runs no Codex, Codex reports no skill load, and Codex can read outside its workspace.
    Each has a probe whose verdict does not depend on chance:
    - the trace formats: trim a live trace of each harness under rule 21 and check that it gives no finding;
    - detection: run on each harness a prompt that names the skill and asks for its first rule, and one about an unrelated subject, and check that the detector reads a load in the first and none in the second, allowing one rerun of a miss;
    - the gaps: look in `claude plugin eval --help` and its docs for a way to grade a saved transcript; run `claude plugin eval` on a case whose project's `AGENTS.md` holds a marker, with a prompt that asks the model to repeat any marker in its project instructions, and check that no run repeats it; check that `codex exec --json` has no item type for a skill; run a Codex prompt that reads a file outside the workspace.
