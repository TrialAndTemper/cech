# Notes for the runner and the release check

Working notes, not rules: the change that builds `cech eval`'s runs (ADR 25 item 6) and the release check (ADR 26) starts from them and writes its own spec.
They come from the probes in [research/probes.md](research/probes.md).

## Running each harness

- Claude Code: `claude -p "<prompt>" --output-format stream-json --verbose --model <model> --permission-mode dontAsk --allowedTools <tools> --strict-mcp-config`, stdin from `/dev/null`, with `CLAUDE_CONFIG_DIR` and `HOME` both throwaway.
  The arm with the plugin installs it with `claude plugin marketplace add <dir>` and `claude plugin install <plugin>@<marketplace>` into that config dir.
  The root `AGENTS.md` loads at start, also in a fresh config dir; a nested one arrives as extra context after a tool call, and Haiku treated it as instructions in only one of three runs.
  A fresh config dir still lists account-level skills, which compete with the skill under test.
  In `dontAsk` mode some read-only `git` shell commands ran although `Bash` was not allowed.
- Codex: `codex exec --json --ephemeral --skip-git-repo-check -s <sandbox>`, stdin from `/dev/null`, with `CODEX_HOME` and `HOME` both throwaway; Codex also reads skills from `$HOME/.agents/skills`.
  The arm with the plugin installs it with `codex plugin marketplace add <dir>` and `codex plugin add <plugin>@<marketplace>`.
  It reads `AGENTS.md` from the repository root down to where it starts, never below.
  A login whose organisation enforces approval on request forces that policy on every run.
  `exec` cannot answer an approval, so commands that need one fail; a run that must write files needs a login without that policy, or an API key.
  With `--ephemeral`, the model id appears nowhere in the output.
- Run the Codex runs one at a time on one login; concurrent runs failed with HTTP 400 in an earlier spike.

## Building the project

- The runner copies `project/` and commits it as the first commit of a new git repository.
  It then copies the case's `project.staged/`, or else the plugin's, on top and stages it with `git add`, so the run sees a staged change.
- For a routed run, it appends the rows of `routing.md` to the table under `## Routing` in the root `AGENTS.md`; an unrouted run uses `project/` as it is.

## Isolation

- Codex's sandbox does not stop reads: a run without the plugin searched `/tmp` with `rg`, found the other arm's installed `SKILL.md`, and read it.
  Each arm needs its own directory tree that the other cannot reach, which in CI means a container per run; locally, separate roots and no install of the plugin anywhere the arm without it can read.
- A Claude Code run without the plugin whose `AGENTS.md` names the skill calls `Skill` and gets "Unknown skill"; the detector's check of the result keeps that from counting as a load.

## What the probes showed

The full results are in [research/probes.md](research/probes.md), and the survey of existing runners in [research/runners.md](research/runners.md).
For the runner:

- A trailer the skill asks for appeared in Codex replies that never read the skill, copied from the description in the skill list; a result grader alone does not prove a load.
- Models often added commentary after the drafted text, so a grader of the reply's last line fails runs that followed the rule.
- Many content runs checked for a staged change, found none and stopped; the fixture project needs a real staged change.

## Reporting

- An error makes a run "no result", never a fail; a run that ends without a result line or agent message, or with a non-zero exit of the harness, is an error.
- The report records the model ids where the harness gives them, the Claude Code, Codex and `cech` versions, the kind of credential, never the credential, and the commit.
- The roles eval that CONTRIBUTING.md asks of a skill that uses a role needs `.cech/roles.toml` in the project: a case's `project.staged/` can carry it.
- `claude plugin eval . --max-cost-usd 0` checks cases against its own rules with no credentials and no cost, but exits 2 either way and reports load errors only on stderr: useful to an author, not as a gate.
