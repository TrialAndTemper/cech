# 5. Native harness features first

- Status: accepted
- Date: 2026-10-16
- Deciders: Michał Bożek

## Context

Things that once needed scripts are now built into Claude Code and Codex: per-project plugin enablement, marketplaces declared in project settings, Codex reading Claude-shaped plugins, `claude plugin validate` and `claude plugin eval`.
Custom copies of such features can break and conflict with the harnesses' own handling.
Harness behaviour can also change between releases.
A workaround is easier to delete when its code names the record of the gap it covers.
A typo in a hand-written data file should fail, not be ignored.
`go:embed` builds in only files in the package's own folder or below, and `go test` leaves files outside the module out of the key of its result cache (checked on 2026-10-29 with Go 1.27.3).

## Decision

1. Before writing CECH code, check whether a harness already does it.
   If so, use it and add only what is missing.
2. Each workaround for a harness gap is recorded once, in the harness data file `cli/internal/harnessdata/harnesses.toml`, with the gap it covers and the probe that shows when to delete it.
3. The file also lists each behaviour CECH relies on, the probe that confirms it, and the last verified harness version.
   The `cech` CLI builds it in, so it is product data, not dev-tool config, and a marketplace that extends CECH needs no copy of it.
   Docs or `--help` cited there carry date and version.
   A pull request that starts relying on a behaviour adds it.
4. A scheduled CI job runs each entry's probe on the newest releases.
   It opens an issue when a result differs from the one expected: a behaviour changed or a gap closed.
   The `cech` CLI warns when an installed harness is newer than the versions recorded there, but never blocks; the scheduled job catches breaks.
5. **The format of the harness data file.**
   Each entry is a TOML table named after the entry, so TOML itself rejects two entries with one name.
   Code that works around a gap cites that name.

   ```toml
   [codex-plugin-agents]
   harness = "codex"
   kind = "gap"
   summary = "Codex leaves out the agents that a plugin bundles."
   workaround = "cech build writes each reviewer agent as a Codex agent file, which cech copies into the project."
   decision = "ADR 6 item 3"
   probe = "Install a plugin that bundles a skill and an agent, then render the prompt with codex debug prompt-input."
   expect = "The prompt names the skill but not the agent."
   verified = { version = "0.165.0", date = 2026-10-29 }

   [[codex-plugin-agents.sources]]
   source = "https://learn.chatgpt.com/docs/agent-configuration/subagents"
   read = 2026-10-29
   version = "0.165.0"
   ```

   | Key | Required | Meaning and rules |
   | --- | --- | --- |
   | the table's name | yes | The entry's name: its `harness` and a hyphen, then parts of lower-case letters and digits joined by single hyphens |
   | `harness` | yes | `claude-code` or `codex` |
   | `kind` | yes | `gap` for a workaround (item 2), `behaviour` for a behaviour CECH relies on (item 3) |
   | `summary` | yes | The gap or the behaviour; not blank |
   | `workaround` | for a gap only | What CECH does instead; not blank, and absent from a behaviour |
   | `decision` | no | The ADR item that sets the workaround or relies on the behaviour, as `ADR N item M` |
   | `probe` | yes | Steps that check the entry on one harness version; not blank |
   | `expect` | yes | The probe's result while the entry holds; not blank; any other result means a gap may have closed or a behaviour changed |
   | `verified` | yes | The last run that gave `expect`: the harness `version` as `X.Y.Z` with no leading zeros, and its `date` as a TOML local date such as `2026-10-29` |
   | `sources` | no | The docs or `--help` the entry rests on, each a `[[<entry>.sources]]` table with a `source`, the `read` date and the harness `version` it was read for; a `source` is an `https://` URL, or a command of the entry's own harness, `claude` or `codex`, alone or followed by a space and its arguments |

   Reading the file fails on an unknown key, a missing key, or a value of the wrong type or form, and lists every such problem, each naming the entry and the key.
   A test of the CLI holds the committed file to these rules, so the merge gate fails on a wrong entry.
   A probe is prose for now; the scheduled job of item 4 adds a key for a runnable probe.

## Consequences

- Less CECH code to break.
- A workaround is deleted once its harness closes the gap.
- Until the scheduled job lands, a person runs the probes by hand, and a probe is only as exact as its prose.

## Options considered

- **A harness-independent layer over both harnesses:** more code, always behind.
- **Trust docs without probes:** docs can lag behind behaviour.
- **The file in `data/`, beside other repository data:** the CLI could not build it in without a copy step that can drift, and a local test run could report a cached pass after the file changed.
- **Entries in an array, each with a `name` key:** TOML would allow two entries with one name, so the code would have to check it.
- **Runnable probe scripts from the start:** they need logged-in harnesses, which the merge gate lacks, so nothing would run them until the scheduled job exists.
