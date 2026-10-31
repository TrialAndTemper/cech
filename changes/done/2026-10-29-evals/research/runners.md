# Existing eval runners

Checked on 2026-10-29 from each tool's source, documentation and release notes, without running a model.
An earlier spike on 2026-10-16 ran the same small suite through `claude plugin eval`, Harbor 0.24.0 and skill-up 0.13.0 on both harnesses; its findings are noted where they still hold.

## What CECH needs

- The plugin installed natively, as a user would install it, on Claude Code and Codex.
- A project with its `AGENTS.md` files, routed or not.
- A skill load detected on both harnesses.
- Subscription logins, including one whose organisation enforces Codex approval on request.
- A run that errors counted as no result, not as 0.

## Candidates

| Tool | Version, date, licence | Harnesses | How the skill gets in | Load detection | Credentials | Main gap |
| --- | --- | --- | --- | --- | --- | --- |
| `claude plugin eval` | Claude Code 2.1.312 | Claude Code | Native plugin install | `Skill` call | Login or key | Claude Code only; loads no project instructions, so no routed runs |
| Harbor | 0.25.0, 2026-10-25, Apache-2.0 | About 50 agents | Skill folder copied in | None | Login token or `auth.json` | Starts Codex with approvals and sandbox bypassed, which a managed login refuses; telemetry on by default |
| BenchFlow | 0.7.11, 2026-10-16, Apache-2.0 | Several, through protocol adapters rather than the CLIs | Skill folder copied in | Not for Codex shell reads | Host logins | Adapters, not the real CLIs; no plugin install |
| promptfoo | 0.125.1, 2026-10-25, MIT | Claude Code and Codex through their SDKs | Claude Code: local plugin; Codex: workspace skills only | `Skill` call; Codex by heuristic | Existing logins | SDKs, not the CLIs; no Codex plugin; `AGENTS.md` loading undocumented |
| inspect_swe | 0.2.72, 2026-10-20, MIT | Several, in a container | Skill folder copied in | None | API keys, through a model proxy | No logins; no plugins |
| skillgrade | 0.5.0, 2026-10-11, MIT | Several | Workspace skill folders | Codex by command path | API keys | No logins; no plugins |
| skill-up | 0.14.0, 2026-10-20, Apache-2.0 | Several | Workspace skills | Misses Codex loads | Host logins | In the spike, on 0.13.0, it detected one of four real Codex loads; runs Codex with approvals bypassed |

None installs a plugin natively on both harnesses and detects a Codex load, and none was confirmed to count an errored run as no result.

## Findings from the spike that still hold

- Harbor's Codex runs failed on a managed login, and it folded infrastructure errors into the mean score.
- skill-up's Codex parser reads only some item types, so most loads went unseen.
- The spike's own Codex arm detected one load in six; the probes in [probes.md](probes.md), detecting the skill's text in command output, found 17 in 18 Codex runs of the matched-wording prompts, routed and unrouted, so the low figure came from the detector.

## Harness facts behind the gaps

- In Codex 0.166.0, the newest release on the day of the check, the `exec --json` items are agent messages, reasoning, commands, file changes, tool calls, web searches, to-do lists and errors: no skill item.
  A structured skill-read tool exists behind a feature flag that is off by default.
- Claude Code reads `AGENTS.md` when a project has no `CLAUDE.md`, and a nested one when it reads a file beside it.

## Smaller projects seen

eval-magic 0.11.1 and agent-skill-eval 0.7.0 run with and without a skill on several harnesses; each has one or two contributors, so they are prior art, not dependencies.

## What CECH takes from them

- The two-arm design, as most of these tools use.
- Recording the effective configuration of each run.
- Harbor and BenchFlow are the ones to recheck: either could gain native plugin installs and Codex load detection.
