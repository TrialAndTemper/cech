# Probes of skill loading and routing

Probes on 2026-10-29 with Claude Code 2.1.312 and Codex CLI 0.165.0, on subscription logins copied into throwaway homes.
Claude Code ran `haiku`, reported as `claude-haiku-5-5`; Codex ran `gpt-6-luna` at reasoning effort `low`; with `--ephemeral`, its output names neither.
Each probe ran three times per prompt, so the rates show what works and how the detectors behave, not effect sizes.

## Fixture

A plugin with one skill and a made-up rule: every commit message ends with `Change-Kind: feat|fix|chore`.
Its description names commit messages; its body holds a marker string that appears nowhere else, so a run that read the body shows it.
The plugin had both manifests and was installed natively from a local marketplace into the throwaway home of the arm with the plugin.

Each run started in a fresh copy of a small git project:

- *unrouted*: a root `AGENTS.md` with no routing;
- *routed*: the same file plus a routing row that sends commit messages to the skill.

The second probe used a fresh plugin and skill name, so no install or cache from the first could answer for it.

## Detectors

- Claude Code: a `Skill` tool call naming the skill whose result is not an error.
  A call on the tool alone is not enough: without the plugin, a routing row naming the skill led to `Skill` calls whose result was "Unknown skill" with `is_error` true.
  A successful load's result had no `is_error` key.
  Installed from a plugin, the skill is called as `<plugin>:<skill>`.
- Codex: the body marker in the output of a `command_execution` item.
  `codex exec --json` has no skill event; a load shows only as a shell command that prints `SKILL.md`.
  The command's exit status does not matter: two loads chained the read with a command that failed.
  Matching the command's path instead missed a run whose model mistyped the path, and counts reads of any copy on disk.
  Re-scored on the first probe's 42 saved Codex runs, a detector that looks instead for the `name:` line and the first line of the description, as the spec defines it, agreed with the marker on every run, 18 loads in all.
  Unlike the marker, it misses a read that skips the frontmatter, and counts a search that prints only the frontmatter of the skill's `SKILL.md`.

The reply's text is no detector: Codex runs that never read the skill still ended with the trailer, copied from the description in the skill list.

## Matched wording

Should-load prompts asked for commit messages, in the description's words; skip prompts asked a git question, a Python question and a trivia question.

| Harness | Unrouted, should load | Routed, should load | Skip prompts, either |
| --- | --- | --- | --- |
| Claude Code | 9/9 | 9/9 | 0/18 |
| Codex | 8/9 | 9/9 | 0/18 |

The description alone took both harnesses to the ceiling, so routing had nothing to add.
The one Codex miss tried to load the skill and failed on a mistyped path.

## Oblique wording

Should-load prompts asked for the same thing in other words: a record for the project history, a staged change for the log, the description to save with a change.
Skip prompts were near misses: explaining an interactive rebase, a rename with its diff, and a changelog entry.

| Harness | Unrouted, should load | Routed, should load | Near misses, either |
| --- | --- | --- | --- |
| Claude Code | 6/9 | 9/9 | 0/18 |
| Codex | 3/9 | 4/9 | 0/18 |

Per prompt, loads out of three, unrouted then routed: Claude Code 1→3, 2→3 and 3→3; Codex 1→2, 2→2 and 0→0.

- Routing raised Claude Code's loading from 6 to 9 of 9; Codex's change, 3 to 4, is within noise at nine runs.
- Routing added no false loads.
- The Claude Code difference is not significant at this size: a one-sided Fisher exact test gives p ≈ 0.10.
- Whether Codex's weak response comes from the model at low effort or from the routing text is open.

## Runs without the plugin

Each probe ran the prompts once per harness without the plugin, in the routed project.

- In the first probe, a Codex run without the plugin, in a read-only sandbox, searched the shared temporary folder, found the other arm's installed `SKILL.md` and read it.
  The sandbox limits writes and some commands, not reads.
- In the second probe, the runs without the plugin ran before the plugin source or any install existed on disk, and each arm had its own parent folder: none of them loaded the skill or printed the marker.

## AGENTS.md loading

- Claude Code read the root `AGENTS.md` at start, including the first session in a new config folder.
  A nested `AGENTS.md` arrived as extra context after a tool read a file beside it; one run of three treated it as instructions, two as tool output.
- Codex, checked with `codex debug prompt-input`, which renders what the model sees without a model call, read `AGENTS.md` from the project root down to the folder it started in, and never one below.

## Other findings

- Content runs often checked for a staged change, found none and stopped, though they named the rule; a content fixture needs a real staged change.
- Models often added commentary after the drafted text, so a grader of the reply's last line fails runs that followed the rule.
- A new Claude Code config folder still listed the account's own skills, and the list changed between runs.
- With `--permission-mode dontAsk`, some read-only `git` commands ran although `Bash` was not allowed.
- On a login whose organisation enforces approval on request, `codex exec` cannot answer approvals, so commands that need one fail; some reads ran without one.
- With `--ephemeral`, Codex leaves no record of the model it used.
- Cost: about $0.003 a run on Claude Code; about 1.4 million input tokens, most cached, for 42 Codex runs, with no price reported.
- Credential files were copied per arm, never rewritten by a run, and deleted after; the originals were unchanged.

## How to rerun

These are the exact texts and commands, with paths as placeholders; the probes ran them by hand, with no script.

### The skill

The second probe's `SKILL.md`; the first used the name `cech-demo` and another marker.

```markdown
---
name: cech-tidy
description: Use when writing or reviewing a commit message. Ends it with the project's Change-Kind trailer.
---

# Commit message house rule

Marker: PROBE-TIDY-BODY-5208

Every commit message in this project ends with a final line of the form:

    Change-Kind: feat|fix|chore

Pick `feat` for new behaviour, `fix` for a bug fix and `chore` for anything else (renames, dependency bumps, tooling).
The trailer is the last line of the message, after a blank line.
```

The plugin of the same name holds only this skill, with a Claude Code and a Codex manifest, in a local marketplace with a manifest for each harness.

### The project

A git repository with one commit, holding `README.md` (`# Pager` and one line), `src/pager.py` (one function that splits a list into pages) and this root `AGENTS.md`:

```markdown
# Agent operating contract

## Where am I?

The root of Pager, a tiny Python library that splits lists into pages.

## What belongs to this layer?

Everything in the repository: `README.md` and the code in `src/`.

## Routing

Load each matching SKILL.md before drafting.

| Change | Destination | Rule or reference |
| --- | --- | --- |
| Recording a change in git history | `git commit` | the `cech-tidy` skill: load its SKILL.md first |

## How to work

Verify a change with `python3 -c "import sys; sys.path.insert(0, 'src'); import pager; print(pager.page([1,2,3], 2))"`.
```

The unrouted project drops the `## Routing` section.
The first probe's routing row was the one below, and its routed project also had a `src/AGENTS.md` routing commit messages for `src/` to the skill.

```markdown
| A commit message | `git commit` | the `cech-demo` skill |
```

### The prompts

Matched wording:

- Load: "Write a commit message for my staged change: I fixed an off-by-one in the pager.
  Reply with the message only."
- Load: "Draft the commit message for renaming getUser to fetchUser across three files."
- Load: "What should the commit say for bumping the logging library to 2.4?"
- Skip: "How do I undo the last git rebase?"
- Skip: "Reverse a list in Python."
- Skip: "What is the capital of Portugal?"

Oblique wording:

- Load: "I've staged a fix for the off-by-one in the pager.
  Write the text I should record for it in the project history.
  Reply with that text only."
- Load: "Prepare my staged change for the log: a summary line and a short body."
- Load: "I'm about to push the rename of getUser to fetchUser.
  Give me the description to save with the change."
- Skip: "Explain what `git rebase -i` does, in two sentences."
- Skip: "Rename getUser to fetchUser in src/pager.py and show me the diff only, no other text."
- Skip: "Write a CHANGELOG entry for the pager fix, in Keep a Changelog style."

### The commands

Each run starts in a fresh copy of the project, with stdin from `/dev/null`, a throwaway `HOME`, and a copy of the login in the harness's throwaway home.
Run the arm without the plugin first, before the marketplace exists on disk, and keep each arm under its own parent folder.

```sh
# Claude Code; before each run, the arm with the plugin runs, in the same config folder:
#   claude plugin marketplace add <marketplace>
#   claude plugin install cech-tidy@<marketplace-name>
HOME=<home> CLAUDE_CONFIG_DIR=<config> claude -p "<prompt>" \
  --output-format stream-json --verbose --model haiku \
  --permission-mode dontAsk --allowedTools Read Glob Grep Skill \
  --strict-mcp-config </dev/null

# Codex; before each run, the arm with the plugin runs, with the same CODEX_HOME:
#   codex plugin marketplace add <marketplace>
#   codex plugin add cech-tidy@<marketplace-name>
HOME=<home> CODEX_HOME=<codex-home> codex exec --json --ephemeral \
  --skip-git-repo-check -s read-only -m gpt-6-luna \
  -c model_reasoning_effort="low" "<prompt>" </dev/null
```

Score each run with the detectors above, on the saved output.
