# Evals and the eval gate

Implemented 2026-11-18; not maintained: the code and its tests are current.

What a plugin's eval folder holds, what `cech eval --check` checks, and how the eval gate runs it.
The ADRs behind it are ADR 5 item 5, ADR 13 item 4, ADR 15 item 5, ADR 16 item 1 and ADR 25.
It builds on the change that added `cech build` (ADR 18 items 3 and 7), which reads the plugins under `plugins/`; that change also added the YAML module.
The runner, the release check and the script grader that ADR 25 item 4 allows are not part of this change; [brief.md](brief.md) holds notes for them.
The two harnesses are named `claude-code` and `codex` in files and messages.

## Layout

1. A plugin's eval folder is `evals/<category>/<plugin>/` at the marketplace root, where `plugins/<category>/<plugin>/` is a plugin folder as `cech build` reads it.
   It holds `project/`, optionally `project.staged/`, `routing.md`, and one folder per case.
2. `evals/` holds nothing else, apart from `evals/results/`, which `.gitignore` lists.
   Anything else in `evals/`, including a `mocks/` folder at any depth, is a finding.
   So is a `prompt.md` or `case.yaml` anywhere in `evals/` other than at the top of a case folder, since `claude plugin eval` would load it as a case.
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

7. A case folder holds `prompt.md` in the format of `claude plugin eval`, `graders/` with at least one `<name>.md`, `cech.toml`, and `recordings/`.
   It may hold `case.yaml`, the fixture files and scripts it names, any file a grader names, such as a `baseline_file`, and `project.staged/`, which replaces the plugin's for this case.
8. A case's name is its folder's name, other than `project` and `project.staged`.
   `prompt.md` has no `name`, and a `case.yaml` has `name` equal to the folder's name.
   Case names are unique across the marketplace.
9. `prompt.md` starts with YAML frontmatter, whose `plugins` is exactly `["../../../../plugins/<category>/<plugin>"]`, naming the case's own plugin.
   `case.yaml` has only `schema_version`, `name`, `description`, `tags` and `context`, and a content or control case has no `context.history_file`, which `claude plugin eval` runs in one arm only.
   Every other key is left to `claude plugin eval`.

## Graders

10. Each grader file starts with YAML frontmatter, with `type` one of `regex`, `tool_used`, `tool_order`, `file_exists`, `llm` or `baseline`.
    Its keys are those its type takes, plus `weight`, a positive number, and `arm`, `with-only` or `both`:

    | Type | Keys |
    |---|---|
    | `regex` | `pattern` (required), `flags`, `match`, `target` |
    | `tool_used` | `tool` (required), `input_match`, `min`, `max` |
    | `tool_order` | `before` and `after` (required): a tool name, or a map of `tool` and an optional `input_match` |
    | `file_exists` | `path` (required), `exists` |
    | `llm` | `criteria` or a body, not both, and `focus` |
    | `baseline` | `baseline_file` (required), `criteria` |

    Any other key or value is a finding.
11. `match` is `contains` (the default), `not_contains` or `count:N`, with `N` a whole number.
    `target` is `last_message` (the default), `files`, or a map `{source: file, path: <path>}` with a relative path.
    `trace` and `mock_calls` are findings: a live trace holds lines and paths that a recording leaves out, and mocks are not allowed.
    `flags` holds only `i` and `s`, each at most once.
    `min` and `max` are whole numbers, `min` at most `max`; `exists` is a boolean.
    `path` in `file_exists` is a glob of `*`, `?` and `**` over slash-separated relative paths; any other glob character, and an empty, `.` or `..` part, is a finding.
12. `regex` and `file_exists` graders read the result and apply on both harnesses.
    `tool_used` and `tool_order` graders name Claude Code tools and apply on `claude-code` only.
    `llm` and `baseline` graders call a model and are not graded offline.
13. Each content and control case has a `regex` or `file_exists` grader.
    A trigger case's graders play no part in its verdict, which is the load the runner detects (rule 24); it keeps one grader, so `claude plugin eval` can still run it: a `tool_used` grader with `tool: Skill` and an `input_match` that matches the skill as `<skill>` or `<plugin>:<skill>` and no other name, with `min` at least 1 for `expect = "load"`, and `min` and `max` 0 for `expect = "skip"`.

## Regular expressions

14. Every `pattern` and `input_match` uses only this subset of JavaScript regular expressions, without the `u` flag:
    - literal characters, and `^` and `$`;
    - `.`, which `cech` reads as JavaScript does: any character but `\n`, `\r`, U+2028 and U+2029, or with `s` any character;
    - the escapes `\d`, `\D`, `\w`, `\W`, `\b`, `\B`, `\n`, `\r`, `\t`, and of the characters `\^$.|?*+()[]{}/`;
    - `\s` and `\S`, which `cech` reads as JavaScript does, with `\v` and the Unicode spaces;
    - bracket classes, plain or negated with `^`, of literal characters, ranges, `\d`, `\D`, `\w`, `\W`, `\s`, `\n`, `\r`, `\t`, and escapes of the characters above and `-`;
    - groups `(...)` and `(?:...)`, and alternation `|`;
    - the quantifiers `*`, `+`, `?`, `{n}`, `{n,}` and `{n,m}`, each optionally followed by `?`.
15. Anything else is a finding: lookaround, back-references, named groups, inline flags such as `(?i)`, `\p{...}`, `\u...`, `\x...`, other escapes, `\b`, `\B` or `\S` in a bracket class, a bracket class that starts with `]` (so also `[^]`), an unescaped `[` in a bracket class, and an unescaped `{`, `}` or `]` outside a repeat or class.
    So is a quantifier with nothing JavaScript can repeat: at the start of a pattern, group or alternative; after `^`, `$`, `\b` or `\B`; or after another quantifier, apart from one lazy `?`.
    So is `\s` followed by `-` and another character in a bracket class, which Go would read as a range once `\s` is spelled out.
    So is a pattern whose literal characters, with escapes removed, include `/cech/`, the root of rule 22's placeholders, since a live run has real paths there.
    Go must compile the pattern as `cech` rewrites it under rule 14.
16. A `regex` grader with `count:N` has a pattern whose every match takes at least one character, judged on Go's parsed syntax tree, since JavaScript and Go count empty matches differently.
17. Two differences remain, and no rule catches them, so CECH's patterns and recordings stay clear of them:
    - with `i`, Go also matches a few characters JavaScript does not, such as U+212A KELVIN SIGN for `k` and U+017F LATIN SMALL LETTER LONG S for `s`;
    - a character outside the Basic Multilingual Plane, such as an emoji, is one character in Go and two in JavaScript, which changes what `.`, a quantifier after it, or a class that holds it matches.

## cech.toml

18. `cech.toml` has these keys, and no others:

    | Key | Families | Required | Value |
    |---|---|---|---|
    | `family` | all | yes | `content`, `control` or `trigger` |
    | `expect` | all | yes | content `better`; control `same`; trigger `load` or `skip` |
    | `wording` | trigger | yes | with `expect = "load"`, `matched` or `oblique`; with `skip`, `unrelated` or `near-miss` |
    | `critical` | content | no | boolean, default false |
    | `claim` | content | yes | the skill rule the case tests, one line, not empty |
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
    With each placeholder removed together with the parts of the path it starts, which end at a character other than a letter, digit or one of `._@+~-`, or at a `..` part, no string in `trace.jsonl`, and no path in `created.txt`, holds a local path: one containing `/Users/`, `/tmp/`, `/private/`, `/var/folders/`, `/home/`, or a drive letter followed by `:\`.
    Paths under `workspace/` are read relative to it, so they hold none.
    The same holds for every line of every other `.jsonl` file in a case folder, such as a `history_file` or `baseline_file`, JSON or not.

## Grading a recording

23. `cech eval --check` grades each content and control recording with the case's graders that apply to its harness (rule 12), as `claude plugin eval` grades a run:
    - `last_message` is the `result` of the `result` line on `claude-code`, and the `text` of the last `agent_message` on `codex`;
    - `files` is the paths of the created files, one per line, sorted;
    - `{source: file, path: P}` is the content of `workspace/P`, and the grader fails when it is missing;
    - `regex` passes when the pattern matches the target; with `not_contains`, when it does not; with `count:N`, when it matches exactly `N` times without overlap;
    - `tool_used` counts `tool_use` blocks whose `name` is `tool` and whose `input`, as written in the trace with whitespace outside strings removed, matches `input_match`; it passes when the count is at least `min` (default 1) and at most `max` (default unlimited);
    - `tool_order` passes when both match some `tool_use` block and the first match of `before` comes before the first of `after`;
    - `file_exists` passes when a created file matches `path`, or, with `exists: false`, none does.

    `weight` and `arm` play no part.
    A recording marked pass is a finding unless every applicable deterministic grader passes on it, and one marked fail is a finding unless at least one fails.
24. A trigger case's recording is graded by the runner's load detector instead:
    - on `claude-code`, the skill loads when a `tool_use` named `Skill`, whose `input` `skill` is `<skill>` or `<plugin>:<skill>`, has a `tool_result` that is not an error;
    - on `codex`, the skill loads when the `aggregated_output` of one `command_execution` has a line ending with `name: <skill>` and a line ending with the first line of the `description` value as `SKILL.md` writes it (for a block scalar, its first content line), matched as written, quotes included.
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
    With no `plugins/` and no `evals/`, it passes with nothing checked, and says so on stderr, or with `--json` reports no cases.
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
35. The harness data file records, as behaviours CECH relies on, the grading of rule 23 and the load detection of rule 24 on each harness, and, as gaps with what CECH does instead: `claude plugin eval` loads no project instructions and runs no Codex, Codex reports no skill load, and Codex can read outside its workspace.
    Each has a probe whose verdict does not depend on chance:
    - grading on `claude-code`: run one content case with `claude plugin eval --json --keep-temp`, trim its trace under rule 21, and compare each grader's verdict with the native one's;
    - the trace formats: trim a live trace of each harness under rule 21 and check that it gives no finding;
    - detection: run on each harness a prompt that names the skill and asks for its first rule, and one about an unrelated subject, and check that the detector reads a load in the first and none in the second, allowing one rerun of a miss;
    - the gaps: run `claude plugin eval` on a case whose project's `AGENTS.md` holds a marker, with a prompt that asks the model to repeat any marker in its project instructions, and check that no run repeats it; check that `codex exec --json` has no item type for a skill; run a Codex prompt that reads a file outside the workspace.

## Tests

The Go tests are in `cli/internal/app/eval_test.go` unless named otherwise; `Findings/"x"` is a case of `TestEvalCheckFindings`, and `Passes/"x"` one of `TestEvalCheckPasses`.

| Rule | Test |
| --- | --- |
| 1 | `Findings/"file loose in a plugin's eval folder"` |
| 2 | `cli/internal/evals/check_test.go` `TestCheckResultsCategory`; `Findings/"mocks"`, `/"nested case"`, `/"eval folder without a plugin"`, `/"eval folder in another category"` |
| 3 | `Findings/"no project"` |
| 4 | `Findings/"no routing table"`, `/"AGENTS.md names the skill"`, `/"CLAUDE.md in the project"` |
| 5 | `Findings/"local path in the project"`, `/"local path in a staged change"` |
| 6 | `Findings/"an escaped bar"`, `/"a header in routing.md"`, `/"no row names the skill"` |
| 7 | `Findings/"no prompt.md"`, `/"no grader file"`, `/"no cech.toml"` |
| 8 | `Findings/"name in prompt.md"`, `/"case.yaml with another name"`, `/"case names repeat across plugins"` |
| 9 | `Findings/"plugins names another folder"`, `/"case.yaml with graders"`, `/"history in a control case"` |
| 10 | `Findings/"unknown grader key"`, `/"unknown grader type"`, `/"regex without a pattern"` |
| 11 | `Findings/"unknown match"`, `/"trace target"`, `/"min above max"`, `/"glob with braces"` |
| 12 | `Passes/"a baseline grader"`; `TestEvalCheck`, whose Codex recordings pass without the tool graders |
| 13 | `Findings/"no result grader"`, `/"trigger matches a sibling skill"`, `/"skip case without max: 0"` |
| 14 | `cli/internal/evals/regex_test.go` `TestCompile` |
| 15 | `cli/internal/evals/regex_test.go` `TestCompile`, `TestCompileJavaScriptRejects` |
| 16 | `Findings/"count of an empty match"` |
| 17 | By hand, 2026-11-18: no pattern or recording in the fixture marketplace holds U+212A, U+017F or a character outside the Basic Multilingual Plane, searched with `grep -rlP` in a UTF-8 locale |
| 18 | `Findings/"unknown cech.toml key"`, `/"wording not for the expect"`, `/"claim on a control"` |
| 19 | `Findings/"too few load cases"`, `/"no oblique prompt"`, `/"no near miss"` |
| 20 | `Findings/"recording not declared"`, `/"declared recording missing"`, `/"recording named twice"` |
| 21 | `Findings/"untrimmed line"`, `/"result line not last"`, `/"untrimmed Codex item"`; strict reading is the JSON library's own, so no test |
| 22 | `cli/internal/evals/check_test.go` `TestLocalPath`; `Findings/"local path in a line that is not JSON"` |
| 23 | `Findings/"grader always passes"`, `/"count:N counts"`; `Passes/"files sorted"` |
| 24 | `Findings/"an errored Skill call is no load"`, `/"a grep prefix still loads"`; `cli/internal/evals/check_test.go` `TestDescriptionLine` |
| 25 | `Findings/"a pass recording names the detector"`, `/"grader always passes"` |
| 26 | `TestEvalCheckInCurrentFolder`; `cli/internal/app/app_test.go` `TestRun/"eval --json without --check"`, `/"eval --check --json -C empty"` |
| 27 | `TestEvalCheckRejectedSource`, `TestEvalCheckEmpty` |
| 28 | `TestEvalCheckPathOrder`, `TestEvalCheckFindingsInOneFile`, `TestEvalCheckKeepsChecking` |
| 29 | `TestEvalCheckJSONFindings`, `TestEvalCheckEmpty`; `TestEvalCheck` with `eval-check-json.golden` |
| 30 | `TestEvalCheckNoRoot`, `TestEvalCheckUnreadable` |
| 31 | `test/tasks.bats` "check:os runs every gate task but commits", ".gitignore keeps eval results out at any depth"; `test/go.bats` "evals runs the cech built from the checkout, not one on PATH" |
| 32 | `test/tasks.bats` "Markdown checks leave out eval recordings and routing rows", "docs:check task leaves out eval recordings"; the term check's scope is unchanged |
| 33 | `cli/internal/build/codeowners_test.go` `TestCodeOwners` |
| 34 | `TestEvalCheck`; `Findings/"grader always passes"`, `/"Codex needs the description line"` |
| 35 | `cli/internal/harnessdata/harnessdata_test.go` `TestFile`, which reads every entry and its probe; each probe was run on the version its `verified` key names |
