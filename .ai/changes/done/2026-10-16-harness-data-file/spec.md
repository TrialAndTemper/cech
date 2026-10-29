# Harness data file format

Implemented 2026-10-29; not maintained: the code and its tests are current.

The format of `cli/internal/harnessdata/harnesses.toml`, the harness data file of ADR 9.

## Example

```toml
[codex-plugin-agents]
harness = "codex"
kind = "gap"
summary = "Codex leaves out the agents that a plugin bundles."
workaround = "cech build writes each reviewer agent as a Codex agent file, which cech copies into the project."
decision = "ADR 10 item 3"
probe = "Install a plugin that bundles a skill and an agent, then render the prompt with codex debug prompt-input."
expect = "The prompt names the skill but not the agent."
verified = { version = "0.165.0", date = 2026-10-29 }

[[codex-plugin-agents.sources]]
source = "https://learn.chatgpt.com/docs/agent-configuration/subagents"
read = 2026-10-29
version = "0.165.0"
```

## Rules

1. Each entry is a TOML table.
   Its name is the entry's `harness`, a hyphen, then parts of lower-case letters and digits joined by single hyphens.
2. `harness` is required: `claude-code` or `codex`.
3. `kind` is required: `gap` for a workaround, `behaviour` for a behaviour CECH relies on.
4. `summary` is required and not blank: the gap or the behaviour.
5. `workaround` is required and not blank in a gap, and absent from a behaviour: what CECH does instead.
6. `decision` is optional: the ADR item that sets the workaround or relies on the behaviour, as `ADR N item M`.
7. `probe` is required and not blank: steps that check the entry on one harness version.
8. `expect` is required and not blank: the probe's result while the entry holds.
9. `verified` is required: the last run that gave `expect`, an inline table with the harness `version` as `X.Y.Z` with no leading zeros, and its `date` as a TOML local date.
10. `sources` is optional: the docs or `--help` the entry rests on, each a `[[<entry>.sources]]` table with a `source`, the `read` date as a TOML local date, and the harness `version` it was read for.
    A `source` is an `https://` URL, or a command of the entry's own harness, `claude` or `codex`, alone or followed by a space and its arguments.
11. Reading the file fails on an unknown key, a missing key, or a value of the wrong type or form, and lists every such problem, each naming the entry and the key.
12. A test of the CLI holds the committed file to these rules, so the merge gate fails on a wrong entry.
