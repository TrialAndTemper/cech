# Research for eval runs and the release check

Findings for the change that builds `cech eval`'s runs ([ADR 25](../../docs/adr/0025-evals.md) item 6) and the release check ([ADR 27](../../docs/adr/0027-the-release-check.md)).
Each was checked on 2026-11-18 against the source it links, unless it gives another date.
The notes on the runner in [the evals brief](../done/2026-10-29-evals/brief.md) still hold, apart from running Codex one at a time (see [Time and cost of a run](#time-and-cost-of-a-run)).

## Telemetry from the harnesses

Both harnesses can export OpenTelemetry: logs and metrics about each session, sent to an endpoint the user sets.

- Claude Code exports metrics and log events once `CLAUDE_CODE_ENABLE_TELEMETRY=1` and an exporter are set ([monitoring docs](https://code.claude.com/docs/en/monitoring-usage), which name versions up to 2.1.324).
  Trace spans are a beta, behind `CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1`.
  - It logs each skill load (`claude_code.skill_activated`, with `skill.name`) and each plugin load (`claude_code.plugin_loaded`, with `plugin.name`).
    `plugin.version` comes only when the name is shown and the manifest has a version.
    Its token and cost metrics carry `skill.name` and `plugin.name` too.
  - It counts retries (`attempt` on each API request) and reports tokens, cost, time and an error type.
  - It logs a third-party skill or plugin under a placeholder name unless `OTEL_LOG_TOOL_DETAILS=1` is set.
    So CECH's skills stay anonymous in an organisation's data until the organisation turns the flag on.
  - It leaves out prompts, tool details and tool content by default.
- Codex exports log events and metrics when `[otel]` in its config names an exporter ([advanced config](https://learn.chatgpt.com/docs/config-file/config-advanced)).
  - It has a metric for the skills added to a run, `skill.injected`, by skill and status, but no event for each load and no skill version.
  - Its `otlp-http` exporter takes only `protocol = "binary"`, which is protobuf; `otlp-grpc` is the other exporter (read again on 2026-11-26).
  - It records retries (`attempt`) and tokens, and leaves out prompts by default.
- Both report the input tokens of each model request.
  So the tokens a plugin adds are the difference in input tokens between runs with it and without it, taken at the first request for the user's prompt (the first request of the main agent loop).
- Claude Code 2.1.329, checked by runs on 2026-11-20:
  - The `claude_code.api_request` event gives `input_tokens`, `cache_read_tokens` and `cache_creation_tokens` for each request.
    The three are separate, and add up to the whole input.
  - The first request of the main loop is the one with the lowest `event.sequence` whose `query_source` is `sdk`, which is its value under `-p`.
  - The split between the three counts changed between identical runs, but their sum did not.
  - The `system/init` line of `stream-json` output lists the tools and plugins loaded.
  - The first request also changed with things outside the plugin.
    One run had 37 tools and 2,399 more tokens than three runs with 24, because MCP connectors on the account connect late.
    The set of built-in plugins also changed between runs.
    `--bare` would avoid them, but needs an API key and may skip project instructions, which routing needs.
- Codex CLI 0.171.0, read in its source on 2026-11-20 and checked by runs on 2026-11-27 in the eval image:
  - `codex.sse_event` with `event.kind` `response.completed` gives `input_token_count` and `cached_token_count` for each request, but only with `[otel]` set.
    The cached count is part of the input count, so `input_token_count` alone is the whole input.
    `codex exec --json` gives only totals per turn.
  - Each record names its event in the `event.name` attribute.
    The event name field of OTLP, the OpenTelemetry protocol, holds the place in Codex's source that logged it, such as `event otel/src/events/session_telemetry.rs:1187`.
  - Codex sends its model requests over a websocket, so `codex.api_request` comes only for its `/models` lookup.
    One `response.completed` comes for each model request.
  - Before the prompt, Codex sends a request of its own of about 2 input tokens to open the websocket.
    So the first request of the main loop is the first `response.completed` after the `codex.user_prompt` event.
  - With the test plugin, that request's whole input was 14,157 tokens; without it, 14,085.
  - Each record also carries the login's `user.email` and `user.account_id`, which the receiver must not keep.
- Claude Code 2.1.330, checked by runs on 2026-11-27 in the eval image, has the same events and fields as 2.1.329.
  Over 320 pairs of runs, the test plugin added 40 tokens to the first request.
- A description's tokens can also be counted outside a run, but not what a harness wraps around it.
  `tiktoken` maps `gpt-5` models to `o200k_base` (`tiktoken/model.py`, read on 2026-11-20), and Anthropic's [token counting API](https://platform.claude.com/docs/build-with-claude/token-counting) is free but needs an API key.
  Both serve only as a cross-check.
- Neither harness sends telemetry to a marketplace.
  The user or the organisation chooses the endpoint, and managed settings can enforce it.
  A project's `.claude/settings.json` cannot turn telemetry on or choose its endpoint, though it can turn a signal off.
- The OpenTelemetry conventions for generative AI define `gen_ai.skill.*` attributes for loading a skill, on a `load_skill` span, with no skill version ([semantic-conventions-genai](https://github.com/open-telemetry/semantic-conventions-genai), `model/gen-ai/spans.yaml` at commit `d28355d`).
  They are still at `development` status.
- Hooks cannot replace telemetry for skills.
  Neither harness has a hook event for a skill load, and a plugin hook that sends data home would act without the user's consent.

So telemetry cannot rebuild a user's task, and none of it reaches CECH; cases stay synthetic, as ADR 25 has them.
What telemetry gives CECH is a measure of its own runs: with OpenTelemetry on in each run, `cech eval` can record the tokens, cost, time, retries and skill loads of each arm.

## How many runs a result needs

Terms used below:

- A *false alarm* is a check that reports a drop when there is none; a rate of 1 in 20 is the statisticians' α of 0.05.
- A *missed drop* is a check that reports no drop when there is one; a rate of 1 in 5 means a check catches 4 real drops in 5, which statisticians call 80% power.
- A *range* (a confidence interval) is the span of pass rates the runs are consistent with, at a given false-alarm rate.
- A *confidence sequence* is a range that stays valid however often it is read, so a check may look after every five runs.
- A *boundary* is the formula that gives a confidence sequence its width at each count of runs.
- An *arm* is one way of running a case, such as with the plugin or without it.
- A *bar* is a condition a release must meet, such as a pass rate above 80%.
- A *routed* run has the fixture project's routing rows, which tell the agent when to load the skill.
- The *spread* of the results is how much they vary from run to run; pass-or-fail results at a pass rate near 0% or 100% vary little.

Findings:

- Runs of one case tend to pass or fail together, so 20 cases run 5 times count as fewer than 100 independent runs.
  If one arm passes 68 of 100 runs and the other 92, the range for the true difference is about 13 to 35 points, as long as the runs are independent.
  When runs of one case are strongly alike (a correlation of 0.6), it widens to about 5 to 44 points.
  So ranges should be computed per case ([Adding Error Bars to Evals](https://arxiv.org/abs/2411.00640)).
- About 43 runs per arm tell 68% from 92% with a missed-drop rate of 1 in 5.
  A 10-point gap, 80% against 90%, needs about 200 runs per arm.
- These figures were computed for this change, with the usual formulas for a gap between two pass rates (Newcombe and Wilson intervals), a correction for runs of one case being alike, and a two-sided false-alarm rate of 1 in 20.
- A test that a skill is no worse looks only one way, so it needs fewer runs than one that looks both ways.
  At a false-alarm rate of 1 in 20 and a missed-drop rate of 1 in 5, the runs per arm are n = (1.645 + 0.842)² (p₁q₁ + p₂q₂) / d².
  Here 1.645 and 0.842 are the points of the normal distribution for a one-sided false-alarm rate of 1 in 20 and a missed-drop rate of 1 in 5.
  Here p₁ and p₂ are the two pass rates, q = 1 − p, and d is the gap to catch.
  That gives about 67 runs for 92.5% against 77.5%, 155 for 90% against 80%, and 13 for 90% against 50%.
- A sequential test, which stops once the result is clear, needs on average about half the runs of a fixed test with the same error rates (Wald, *Sequential Analysis*, 1947).
  Wald's test decides between two pass rates fixed in advance, so it does not fit two arms whose rates are both unknown.
- A confidence sequence fits two unknown rates, and reading it after every five runs does not raise the false-alarm rate.
  For pass-or-fail outcomes it has simple formulas, such as the normal-mixture boundary (Howard, Ramdas, McAuliffe and Sekhon, [Time-uniform, nonparametric, nonasymptotic confidence sequences](https://arxiv.org/abs/1810.08240), *Annals of Statistics*, 2021).
- At three runs per arm, 2 passes in 3 have a range of about 21% to 94%, so three runs per arm catch only gross failures.
- pass@k, at least one pass in k runs, and pass^k, every one of k runs passing, measure different things.
  A skill that users rely on is judged by pass^k ([Demystifying evals for AI agents](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents)).
- Anthropic's skill-creator, at commit `b0cbd3d`, compares a skill with its previous version, and reports pass rate, time and tokens with their spread ([skill-creator](https://github.com/anthropics/skills/blob/main/skills/skill-creator/SKILL.md)).
- A fixed set of three arms (without the plugin, with it, and with the latest final release), 20 cases and 5 runs on two harnesses is 600 runs a release.

## Time and cost of a run

Measured on 2026-11-26 with Claude Code 2.1.332 (`claude-haiku-5-5`) and Codex CLI 0.171.0 (`gpt-6-luna`, low reasoning effort).
The case was `cech-example-trailer` of the test plugin in `cli/internal/app/testdata/eval/complete/`, routed, with 10 runs per arm one at a time, then several at once with the plugin.
Each run started from a fresh copy of the fixture project with the staged change, a throwaway home and a copy of the login.
The arm with the plugin added the marketplace and installed the plugin first.

| Harness | Arm | Runs | Median time | Fastest to slowest | Median cost or input |
| --- | --- | --- | --- | --- | --- |
| Claude Code | without the plugin | 10 | 7.6 s | 6.6 to 14.5 s | $0.0030 |
| Claude Code | with the plugin | 10 | 9.6 s | 7.8 to 15.9 s | $0.0035 |
| Claude Code | with, 6 at once | 6 | 8.7 s | 7.6 to 10.5 s | $0.0034 |
| Codex | without the plugin | 10 | 19.6 s | 8.5 to 34.3 s | 93,000 input tokens |
| Codex | with the plugin | 10 | 12.5 s | 10.1 to 20.8 s | 46,000 input tokens |
| Codex | with, 4 at once | 4 | 15.7 s | 13.0 to 18.8 s | 61,000 input tokens |

- Installing the plugin took about 0.6 s on Claude Code and 0.1 s on Codex.
- Several runs at once worked on one login: 6 Claude Code runs took 11 s in all, and 4 Codex runs 19 s, with no error.
  The HTTP 400 that the evals brief records for concurrent Codex runs did not come back; more than 4 at once was not tried.
- Codex reported tokens, mostly cached, but no cost.
  With a ChatGPT login the limit is the plan's usage allowance, which was not measured.
- In 9 of the 10 Codex runs without the plugin, Codex searched the disk for `SKILL.md`.
  In 6 it found the plugin in the marketplace folder next to the runs and followed it, as the evals brief warns.
  The arms shared a parent folder, so these runs give a run's time but not the arm's pass rate.
  The search also made them longer than the runs with the plugin.
- Claude Code without the plugin passed none of its 10 runs; with it, Claude Code passed 9 of 10 and Codex 10 of 10.
- Runs whose plugin install failed were dropped and run again; only the second batch is reported.
- The test plugin's task is small, so a real skill's runs take longer.

So a run takes about 10 s on Claude Code and 13 to 20 s on Codex, and runs on one login can overlap; the estimates below take 20 s for Codex.

The table below is for the first design, before the simulated release checks; [Simulated release checks](#simulated-release-checks) gives the counts for the design the spec uses.
It is for a release that changes a plugin with 10 content cases, on each harness:

| Design | Runs | Claude Code, 6 at once | Codex, 4 at once |
| --- | --- | --- | --- |
| A pooled comparison, read per case for large drops | about 140 to 320 | 4 to 9 min | 12 to 27 min |
| Each case shown to be no worse, at 95% pass rates | about 500 to 1,150 | 14 to 32 min | 42 to 96 min |
| Each case shown to be no worse, at 85% pass rates | about 1,300 to 3,000 | 36 to 84 min | 1.8 to 4.2 h |

- The counts use the one-way formula above for each case, with a threshold of 15 points (the low end) or 10 points (the high end), a false alarm once in 20 and a missed drop once in 5.
- They are halved for a sequential test, as Wald's figure suggests, which is only a rough guide for a confidence sequence.
- They are multiplied by about 1.9, so that 10 cases together still raise a false alarm only once in 20.
- The pooled row is one comparison, so it is neither halved nor multiplied.
- They count only the comparison's two arms, at the measured times; a real skill's runs may take two to four times as long, and the whole check then takes two to four times as long too.
- On Claude Code even 3,000 runs cost about $10.

## Where runs happen and on whose account

Read on 2026-11-26, against the pages linked:

- Claude Code: `claude setup-token` makes a one-year OAuth token, set as `CLAUDE_CODE_OAUTH_TOKEN`, on a Pro, Max, Ultra, Team or Enterprise plan ([authentication](https://code.claude.com/docs/en/authentication), [GitHub Actions](https://code.claude.com/docs/en/github-actions)).
  The docs give it "for CI pipelines, scripts, or other environments where interactive browser login isn't available", and runs with it "use your Claude subscription instead of API billing".
  - OAuth is for "ordinary use", and may not be offered to others or route their requests ([legal and compliance](https://code.claude.com/docs/en/legal-and-compliance)).
    For a secret shared across repositories the docs recommend an API key, since a token is tied to one person's plan.
  - Runs count against the plan's usage limits, which these pages do not give for Pro, Max and Ultra ([costs](https://code.claude.com/docs/en/costs)).
- Codex: an API key is "the right way to authenticate automation", billed at API rates ([authentication](https://learn.chatgpt.com/docs/auth)).
  - A ChatGPT login in CI is an advanced path for "trusted private infrastructure", with one `auth.json` per runner or per serialised job stream, kept between runs ([CI/CD auth](https://learn.chatgpt.com/docs/auth/ci-cd-auth)).
  - The same page says: "Do not use this workflow for public or open-source repositories".
  - Before a run, Codex refreshes a login older than about 8 days and writes the new tokens back.
    The docs warn against sharing one file across concurrent jobs or machines.
    So a batch should refresh the login once, then copy it to each run.
  - Access tokens for scripts exist only in ChatGPT Enterprise workspaces.
- Sandcat, Apache-2.0, at commit `405ed10` ([docs](https://sandcat.virtuslab.com/latest/index.html), [repository](https://github.com/VirtusLab/sandcat)), is a Docker dev container.
  All its traffic goes through a mitmproxy proxy with an allowlist, which adds the API keys.
  - It has no releases, so only a commit pins it, and it needs Docker and `yq`; Podman is not mentioned.
  - It is built for one interactive session: running `claude -p` or `codex exec` from a script, and several runs at once, are not documented.
  - Subscription tokens "bypass secret substitution", so the container holds the token itself.
    Its image's tools default to unpinned LTS versions, and its cache volumes are shared across runs.
- Anthropic's [sandbox-runtime](https://github.com/anthropic-experimental/sandbox-runtime) (`srt`, npm 0.0.83) limits one process's file reads, writes and network without a container, through bubblewrap on Linux or `sandbox-exec` on macOS.
  It runs the host's harness versions, and its Windows support is alpha.

What a Codex run loaded, checked on 2026-11-26 with Codex CLI 0.171.0:

- `codex exec --json` names no plugin or skill.
- `codex debug prompt-input`, run with the same home and project, shows what the model sees, without calling the model.
  Its skills list names each installed plugin's skill as `<plugin>:<skill>`, with its description.
- `codex plugin list` and `codex mcp list` name the installed plugins and MCP servers.

Checked on 2026-11-27 on Fedora 44, with Podman and SELinux enforcing:

- SELinux gives each container a label that limits what it may touch.
  A labelled container cannot connect to a Unix socket that a host process listens on, even in a folder relabelled for containers; the check is on the connection, not on the file.
  The same container with its label turned off can.
  So the receiver's socket is reached through a forwarder container of its own, the only one that runs without its label.
- A container on a Podman `--internal` network resolved no outside name: its DNS server, aardvark-dns, did not answer for `example.com`.
  The network's gateway did not accept a connection on port 22.
  Docker's internal networks were not tried.
- Codex ran shell commands in a container with no capabilities and `no-new-privileges`, with its own sandbox off.
  With its read-only sandbox on, its sandbox tool, `bwrap`, could not mount `/dev/pts`, with or without those flags.

So a fresh container per run, from an image that pins the harness versions, keeps the arms apart without relying on a harness's own sandbox, and the same image can run in CI later.
Until CECH has API keys, runs happen on a maintainer's machine, on that maintainer's logins: in CI, Codex would need an API key, and a Claude Code token would tie the repository to one person's plan.

## Simulated release checks

Simulated on 2026-11-26: 600 release checks of one case, each comparing the plugin being released with the latest final release.
Each check used a 15-point threshold and a false alarm once in 20, and read the results after every five model runs per arm.

- The normal-mixture boundary assumes the worst case: that the results vary as much as pass-or-fail results can.
  So it narrows slowly.
  With a cap of 150 runs per arm, and the false-alarm rate split across 10 cases so that all 10 together raise a false alarm only once in 20, an unchanged case at an 85% pass rate passed in only 6% of the checks, and a 30-point drop failed in only 6%.
  All the other checks ended with no result.
- The predictable plug-in empirical-Bernstein sequence (Waudby-Smith and Ramdas, [Estimating means of bounded random variables by betting](https://arxiv.org/abs/2010.09686), *Journal of the Royal Statistical Society B*, 2024) also has a simple formula.
  It narrows with the spread the results show, which is small at high pass rates.
- The table shows the results with this sequence.
  Each row is a cap, with or without the split; each column gives the share of checks that ended as its heading says, and the last gives the mean runs per arm over the simulated pass rates:

| Cap | Split across 10 cases | Unchanged at 85% passes | Unchanged at 95% passes | 30-point drop fails | Mean runs per arm |
| --- | --- | --- | --- | --- | --- |
| 150 | yes | 6% | 12% | 6% | about 149 |
| 300 | yes | 73% | 100% | 45% | 186 to 259 |
| 500 | yes | 98% | 100% | 85% | 187 to 329 |
| 300 | no | 94% | 100% | 77% | 119 to 200 |
| 500 | no | 100% | 100% | 97% | 117 to 211 |

- In no row did an unchanged case fail, or a 15-point drop pass.
  A drop of exactly the threshold had no result in every row, since a bar fails only when the runs show a drop of at least the threshold.
- So splitting the false-alarm rate across cases nearly doubles the runs for almost no change in what passes or fails.
- At a cap of 500 with no split, a changed plugin with 10 content cases takes about 3,300 runs per harness: about 4.5 hours on Codex at 4 runs at once, and 1.5 hours on Claude Code at 6, at the measured run times.

## Choosing the bars

Simulated on 2026-11-27 with the sequence the bars read, a false alarm once in 20, read after every five runs per arm, and a cap of 500 runs per arm.
A rate bar passes reliably only when a skill's true rate sits about 10 points clear of it, and even a perfect skill needs a minimum number of runs:

| Bar | True rate that passes nearly always | Runs per arm for a perfect skill |
| --- | --- | --- |
| Content above 50% | 70% or more | 20 |
| Content above 80% | 90% or more | 40 |
| Content above 90% | 99%; 95% passes 88% of the time | 80 |
| Load above 70%, three cases pooled | 80% or more | 10 |
| Load above 80% | 90% or more; 85% passes 88% of the time | 15 |
| Load above 90% | 95% or more | 30 |
| Wrong loads below 20%, three cases pooled | 10% or less; 15% passes 88% of the time | 15 |
| Wrong loads below 10% | 5% or less | 30 |
| Wrong loads below 5% | 2% or less | 55 |

The drop bar, comparing a case with the latest final release, at two thresholds:

| Change | Threshold 15 points | Threshold 10 points |
| --- | --- | --- |
| None, at 95% to 100% | passes, after 100 to 115 runs per arm | passes, after 150 to 190 |
| None, at 85% | passes, after 165 | passes 78% of the time, after 320 |
| 100% to 90% | passes 59% of the time | no result |
| 95% to 85% | passes 28% of the time | no result |
| 95% to 80% | no result | no result, or fails 16% of the time |

- At 15 points a real 10-point drop often passes; at 10 points it ends with no result, which a person must then accept before a release.
- Anthropic's skill-creator, read on 2026-11-27 (`scripts/run_eval.py`), runs each trigger query 3 times and counts it right when the skill loads in at least half of them, or, for a query that should not load it, in fewer than half; it sets no target pass rate for content.
- On Codex, the test plugin's obliquely worded trigger case loaded in 15 of 25 runs, so its three load cases pooled about 87%, below a load bar of 90%.

## Graders for files a skill writes

- The graders of `claude plugin eval` read text (Claude Code 2.1.312, as the evals brief records): a `regex` on `{source: file}` sees the raw bytes of a zipped format such as `.docx`.
- A script can make most checks of a `.docx`: python-docx reads its paragraphs, tables and page sizes.
  Whether a table fits the page needs a render, such as `soffice --headless --convert-to pdf`, as Anthropic's docx skill does at commit `e4bb5d9` ([docx skill](https://github.com/anthropics/skills/blob/main/skills/docx/SKILL.md)).
- "Flags missing data instead of inventing it" can be checked without a model only when the fixture leaves known fields out on purpose.

## Eval tools

- Google's EvalBench, Apache-2.0, `google-evalbench` 1.21.0 of 2026-10-27 on [PyPI](https://pypi.org/project/google-evalbench/), drives Claude Code and Codex headless, with scorers for skills and for the steps a run took ([repository](https://github.com/GoogleCloudPlatform/evalbench)).
  - A model plays the user.
  - It has no built-in comparison of a run without the skill and with each version.
  - It writes score tables to CSV or BigQuery, not recordings of each run that a grader can check again, as ADR 25 item 4 keeps, and adds no telemetry of its own.
  - It does not change ADR 25's rejection of existing runners, but its scorer for which skills a run loaded is worth comparing with CECH's load detectors.
- OpenAI's Plugin Eval 0.1.2, read on 2026-11-20 in the ChatGPT plugin directory and at its [source](https://github.com/openai/plugins/tree/main/plugins/plugin-eval), is a Node.js CLI for Codex only, not published as a package and without release tags.
  - Its manifest says MIT, but neither the repository nor the plugin folder holds a licence file.
  - `analyze` scores a plugin and its skills offline: manifest fields, frontmatter keys, the length of `description`, a "use when" sentence, broken relative links, file sizes and the tokens a description adds to every session.
  - `benchmark` runs scenarios through `codex exec` with shell-command checks, but has no arm without the plugin, no routing and no repeated runs, and keeps each run's log without grading it again; `compare` diffs two scores.
  - CECH does not use it, for three reasons: mise cannot pin it to an exact version (ADR 5 item 8), its checks are partly its authors' advice and partly Claude Code's frontmatter keys, and none of them is probed against Codex (ADR 9 item 3).
  - Its offline checks of a skill's structure suit `cech validate`, for both harnesses, each Codex rule backed by a probe.
    Its shell checks resemble the script grader of ADR 25 item 4, but run live in the run's folder, not offline against recordings.

## Questions and decisions

The maintainer decided each question, from a recommendation, on the date given, except decisions 21 and 25 to 27, which the author made and showed to the maintainer.
Where a later decision replaces part of an earlier one, both say so.

1. Should runs record tokens, cost, time and retries from the harnesses' telemetry?
   Decided on 2026-11-18: yes, in the run results only, never in recordings ([ADR 27](../../docs/adr/0027-the-release-check.md) item 6, [spec](spec.md) criterion 24).
2. Should a release that changes a skill also run the latest final release as a third arm?
   Decided on 2026-11-18: yes, with a bar on it ([ADR 27](../../docs/adr/0027-the-release-check.md) items 2 and 3, [spec](spec.md) criteria 1, 8 and 23).
3. Should content cases run more than three times, with ranges per case and pass^k in the report?
   Decided on 2026-11-18: runs start at five per arm and grow, up to a cap, until a sequential test is clear; pass^k is left out of the report.
   Decision 17 picks the test and the first values ([spec](spec.md) criteria 1 to 4).
4. Should a grader be able to run a script on the files a run wrote, for formats such as `.docx`?
   Decided on 2026-11-18: yes, checked offline against the recordings like any other grader ([ADR 25](../../docs/adr/0025-evals.md) item 4); it comes with the first plugin that writes such files.
5. Should CECH ship anything that sends telemetry to CECH?
   Decided on 2026-11-18: no ([ADR 28](../../docs/adr/0028-telemetry.md) item 1); the marketplace reference says how an organisation measures CECH's skills in its own telemetry.
6. Should the report show what each skill's description costs, since every session loads it?
   Decided on 2026-11-20: yes, as a size and as the input tokens its plugin adds, not as a bar ([spec](spec.md) criteria 25 and 27).
   The offline checks of a skill's structure that Plugin Eval suggests go to `cech validate`.
7. Should organisations be able to share telemetry, so CECH learns how its skills are used?
   Decided on 2026-11-20: yes, as a summary that `cech` makes from the organisation's own export and the organisation submits by hand ([ADR 28](../../docs/adr/0028-telemetry.md) item 2).
   The command and an issue form for summaries are a later change.
8. When does a content or control run pass, and what is a grader's `weight` for?
   Decided on 2026-11-25: a run passes when every grader of its case passes, the same rule a recording must meet; `weight` is dropped, and the report gives each grader's pass rate ([spec](spec.md) criteria 11, 13 and 27).
9. What is a grader's `arm` for?
   Decided on 2026-11-25: nothing the report does not already show, so `arm` is dropped and every grader runs in every arm ([spec](spec.md) criteria 11 and 13).
10. Should the release check put a bar only on the content cases an author marks `critical`?
    Decided on 2026-11-25: no; its bars read every content case, and `critical` is dropped, since every claim a skill makes must hold ([ADR 10](../../docs/adr/0010-claude-code-and-codex-from-one-source.md) item 4, [spec](spec.md) criteria 5, 8 and 13).
11. What makes a plugin changed?
    Decided on 2026-11-25: any change under its plugin folder since the commit of `main` that the latest final release was cut from, the same commits its changelog lists ([ADR 14](../../docs/adr/0014-releases-and-versions.md) item 4, [spec](spec.md) criterion 22).
    A new plugin has no comparison with a latest final release.
    Decision 12 replaces what this decision said about cases added since that release.
12. Should the comparison with the latest final release pool a plugin's content cases, or test each case?
    Decided on 2026-11-26: each case on its own, since pooling lets a gain in one case hide a large drop in another; a case added since that release is not compared ([spec](spec.md) criteria 1, 4, 8 and 23).
    This decision also split the false-alarm rate across the cases; decision 22 replaces that.
13. Who runs the release check, and on whose account, while CECH cannot pay for API keys?
    Decided on 2026-11-26: a maintainer, on their own machine and logins, with every run in a fresh container from a pinned image, so the same command can run in CI with API keys later ([ADR 26](../../docs/adr/0026-eval-runs-in-containers.md), [spec](spec.md) criteria 14 to 21, 28 and 29).
14. Where does the release check's report go?
    Decided on 2026-11-26: `cech eval` writes a report file from a template built into the CLI, so it keeps one form between releases, and a pull request commits it; the full run results are attached to the release ([ADR 27](../../docs/adr/0027-the-release-check.md) item 6, [spec](spec.md) criteria 13, 26 and 27).
15. Must an extension's releases pass the release check, and how do runs treat a plugin's dependencies?
    Decided on 2026-11-26: the check binds CECH's own releases, and an extension may require it ([ADR 27](../../docs/adr/0027-the-release-check.md) item 1).
    An arm installs the plugin's dependencies, and a changed one makes its dependents changed ([spec](spec.md) criteria 22 and 23).
    This decision also had arms install the plugins a fixture project binds to roles; decision 25 replaces that.
16. What happens to a run that loads a plugin or MCP server it should not, or whose telemetry cannot be read?
    Decided on 2026-11-26: it counts as a run that errors ([spec](spec.md) criteria 12 and 24).
17. Which sequential test do the bars use, how do runs that error count, and with which first values?
    Decided on 2026-11-26:
    - every bar reads a confidence sequence, so a rate bar too passes or fails only on clear evidence ([spec](spec.md) criteria 2 to 4);
    - a run that errors is run once more, and a case in which more than 10% of the runs still error has no result (criteria 9 and 10);
    - first values: a threshold of 15 points, a false alarm once in 20, a missed drop once in 5, a cap of 150 runs per arm per case and a budget of 4,000 runs per harness per release (criterion 33).

    Decision 22 replaces this cap and budget.
18. What are the runner's command line, telemetry capture, created files and image?
    Decided on 2026-11-26:
    - `cech eval` with `--plugin`, `--harness`, `--parallel`, `--json` and `--dry-run`, since the CLI reference names a preview for every command that writes ([spec](spec.md) criteria 28 to 31);
    - `cech eval` receives telemetry over OTLP with the OpenTelemetry project's Go modules (criterion 24);
    - a run's created files are the paths new or changed since the staged change was in place (criterion 11);
    - a workflow builds the image from a Containerfile and publishes it to the GitHub container registry (criteria 18 and 19).
19. Which data may the public run results hold, how is a run's network limited, where do the run settings live, and how is the image published?
    Decided on 2026-11-26:
    - the results are trimmed and scrubbed as recordings are ([spec](spec.md) criterion 26);
    - the only way out of a run is an allowlist proxy that `cech eval` starts (criterion 15);
    - the settings are built into the CLI, with no override for extensions (criterion 33);
    - a workflow publishes the image as a public package when started by hand or when its folder changes (criterion 18).
20. Which credential wins, what counts as an unexpected load, what happens to hangs, interruptions and repeated runs, and what does the latest-release arm run?
    Decided on 2026-11-26:
    - an API key wins over a subscription login ([spec](spec.md) criterion 20);
    - a ChatGPT login is refreshed once before the batch (criterion 21); decision 26 changes how;
    - the batch's arm without the plugin sets what a harness loads by itself (criterion 12);
    - a run past 10 minutes errors (criterion 9);
    - finished runs are reused for the same commit, image and versions, and `cech eval` runs on `HEAD` (criteria 28 and 31);
    - an interrupted check keeps its finished runs (criterion 30);
    - the latest-release arm takes the plugin from the release tag, and the case, fixture and graders from `HEAD` (criterion 23).
21. Which details did the maintainer leave to the author?
    Proposed by the author on 2026-11-26, shown to the maintainer and open to change in review:
    - the false-alarm rate is split per plugin and harness; decision 22 replaces this;
    - the boundary is the normal-mixture one, narrowest at the cap; decision 22 replaces this;
    - the budget goes first to the unclear bar with the fewest runs ([spec](spec.md) criterion 2);
    - created files leave out `.git/` and deletions, which narrows decision 18 (criterion 11);
    - `--parallel` defaults to 6 on Claude Code and 4 on Codex (criterion 33);
    - an unknown `--plugin` is a wrong command line, and no changed plugin exits 0 (criterion 28);
    - missing tags exit 3 (criterion 29);
    - `cech eval --check` checks report files (criteria 13 and 27);
    - the cap is chosen for the missed-drop rate (criterion 33);
    - a dirty tree, or a second `cech eval` on the same results, is refused (criterion 29).
22. Which boundary, cap, split of the false-alarm rate and budget should the bars use, after the simulated release checks?
    Decided on 2026-11-26:
    - the predictable plug-in empirical-Bernstein sequence ([spec](spec.md) criterion 3);
    - a cap of 500 runs per arm per case (criterion 33);
    - each case at the full false-alarm rate, with no split across cases (criterion 4);
    - a budget of 10,000 runs per harness per release (criterion 33).

    This replaces the split of decisions 12 and 21, the boundary of decision 21, and the cap and budget of decision 17.
23. How does the image get Claude Code, whose npm package reserves all rights to Anthropic, and which harness versions does it pin?
    Decided on 2026-11-26: the published image leaves Claude Code out and holds its pinned lockfile, from which `cech eval` installs it on the maintainer's machine, so CECH passes on no copy ([ADR 26](../../docs/adr/0026-eval-runs-in-containers.md) item 1, [spec](spec.md) criteria 18 and 19).
    Both harnesses are pinned to the newest release at least a week old (ADR 5 item 10), newer than the versions of the timing runs.
24. Where may a maintainer keep the credentials, besides the environment?
    Decided on 2026-11-27: in the system keyring too, under the service `cech` with the variable's name as the user name ([spec](spec.md) criterion 20).
    `cech eval` reads it with `github.com/zalando/go-keyring`, which reaches the Secret Service on Linux, the macOS keychain and the Windows Credential Manager.
    The environment wins, so CI keeps using its secrets.
25. What does an arm install when a plugin depends on one the marketplace does not hold, and what of role bindings?
    Decided by the author on 2026-11-27: `cech eval` refuses to run, and installing role bindings and an extension's CECH plugins waits for the change that gives each a format or a source ([spec](spec.md) criterion 29, Out of scope).
    This replaces the role bindings of decision 15.
26. How is a ChatGPT login refreshed without exposing the user's Codex home?
    Decided by the author on 2026-11-27: `cech eval` refreshes a copy of the login in a folder of its own, and puts the copy back in place of the login only when Codex changed it ([spec](spec.md) criterion 21).
    Mounting the Codex home instead relabelled it for SELinux and showed the container everything in it.
    This changes the in-place refresh of decision 20.
27. What ties a run to its image, when `CECH_EVAL_IMAGE` can name a tag that a rebuild moves?
    Decided by the author on 2026-11-27: the image's ID, which the reuse key, `run.json` and the Claude Code install all use, and the report gives beside the name ([spec](spec.md) criterion 31).
28. Which bars and threshold, after [Choosing the bars](#choosing-the-bars)?
    Decided on 2026-11-27:
    - a content pass rate above 80%, since a skill users rely on should hold most of the time, and the cheapest model still has room for its misses ([spec](spec.md) criterion 5);
    - routed loads above 70%, kept, since the oblique case is meant to be hard and pooling lets it pull the rate down (criterion 6);
    - wrong loads below 10%, the skip bar, since one in ten is already noticeable (criterion 6);
    - a threshold of 10 points, measured against the latest final release, not the best release so far, since the content bar already sets a floor and the best of noisy results tends to be a lucky one (criteria 7 and 8).

    This replaces the first values of decision 17 for the bars and the threshold; the cap and budget of decision 22 stay.
    The cap was chosen at a threshold of 15 points.
    At 10 points and the same cap, a drop near the threshold seldom fails: it ends with no result, and passes no more often than the false-alarm rate.
    A person then decides whether to accept it (ADR 27 item 5).
