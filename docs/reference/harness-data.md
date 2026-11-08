# The harness data file

`cli/internal/harnessdata/harnesses.toml` records the harness behaviours CECH relies on and its workarounds for harness gaps ([ADR 9](../adr/0009-native-harness-features-first.md)).
The `cech` CLI builds it in, so an extension needs no copy.
The CLI's tests read the committed file and fail on any finding below, so the Go gates check it.

## Entries

Each entry is a TOML table whose name is `<harness>-<short-name>`: the harness, `claude-code` or `codex`, then parts of lower-case letters and digits joined by single hyphens, such as `codex-plugin-agents`.
Code that works around a gap names its entry, so the workaround can be found and deleted when the harness closes the gap.

| Key | Type | Required | Holds |
| --- | --- | --- | --- |
| `harness` | string | yes | `claude-code` or `codex`, matching the start of the entry's name |
| `kind` | string | yes | `gap`, a harness gap CECH works around, or `behaviour`, a harness behaviour CECH relies on |
| `summary` | string | yes | What the harness does or lacks |
| `workaround` | string | for a gap only | What CECH does instead |
| `decision` | string | no | The ADR item the entry serves, written as `ADR N item M` |
| `probe` | string | yes | How to check the harness, in steps a person can follow |
| `expect` | string | yes | The probe's result while the entry holds |
| `unattended` | bool | no | `true` when the probe runs without a person, as a script in `scripts/probes/<entry name>` |
| `verified` | table | yes | `version`, the last harness version that gave the expected result, as `X.Y.Z`, and `date`, a TOML local date |
| `sources` | array of tables | no | Docs or help text the entry rests on: each with `source`, an `https` URL or a command that starts with the harness's command, `read`, a local date, and `version`, as `X.Y.Z` |

Reading the file fails on, and names, any key not in this table, a key of the wrong type, a missing required key, and a `workaround` on a `behaviour`.

## Probes

A probe is re-run on each new harness release: a scheduled job runs every probe marked `unattended` on the newest releases, and a person runs the others.
A result that matches `expect` updates `verified`.
Any other result means a behaviour changed or a gap closed, and the job or the person opens an issue for it.
