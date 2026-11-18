# Research for eval runs: telemetry, run counts and eval tools

Findings for the change that builds `cech eval`'s runs ([ADR 25](../../docs/adr/0025-evals.md) item 6) and the release check.
Each was checked on 2026-11-18 against the source it links, unless it says otherwise.
The notes for the runner in [the evals brief](../done/2026-10-29-evals/brief.md) still hold.

## Telemetry from the harnesses

- Claude Code exports OpenTelemetry metrics and log events once `CLAUDE_CODE_ENABLE_TELEMETRY=1` and an exporter are set; trace spans are a beta behind `CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1` ([monitoring docs](https://code.claude.com/docs/en/monitoring-usage), which name versions up to 2.1.324).
  - It already logs skill loads (`claude_code.skill_activated`, with `skill.name`) and plugin loads (`claude_code.plugin_loaded`, with `plugin.name` and `plugin.version`), and puts `skill.name` and `plugin.name` on its token and cost metrics.
  - It counts retries (`attempt` on API requests) and reports tokens, cost, time and an error type.
  - A third-party skill or plugin is logged under a placeholder name unless `OTEL_LOG_TOOL_DETAILS=1` is set, so CECH's skills are anonymous in an organisation's data until that organisation turns the flag on.
  - Prompts, tool details and tool content are left out by default.
- Codex exports log events and metrics when `[otel]` in its config names an exporter ([advanced config](https://learn.chatgpt.com/docs/config-file/config-advanced)).
  - It has a metric for skills injected into a run, by skill, but no event per load and no skill version.
  - It records retries (`attempt`) and tokens, and leaves out prompts by default.
- Both report input tokens per model request, so the tokens a plugin adds show as the difference in a run's first request of the main agent loop between the arms with and without it.
  - Claude Code 2.1.329, checked by runs on 2026-11-20: the `claude_code.api_request` event gives `input_tokens`, `cache_read_tokens` and `cache_creation_tokens` per request, which are separate and add up to the whole input; the first request of the main loop is the one with the lowest `event.sequence` whose `query_source` is the main thread (`sdk` under `-p`).
    The split between the three changed between identical runs, but their sum did not; the `system/init` event of `stream-json` output lists the tools and plugins loaded.
  - In those runs the first request also changed with things outside the plugin: one run had 37 tools and 2399 more tokens than three runs with 24, because account-level MCP connectors connect late, and the set of built-in plugins changed.
    Turning the connectors off should leave the plugin as the only difference, but a clean pair was not yet measured; `--bare` needs an API key and may skip project instructions, which routing needs.
  - Codex, read in its source on 2026-11-20 but not run: `codex.sse_event` on `response.completed` gives `input_token_count` and `cached_token_count` per request, only with `[otel]` set; `codex exec --json` gives totals per turn only.
    The cached count is taken to be part of the input count, as in OpenAI's API usage, so the whole input is `input_token_count` alone; a run should confirm it.
  - `tiktoken` maps `gpt-5` models to `o200k_base` (`tiktoken/model.py`, read on 2026-11-20), and Anthropic's [token counting API](https://platform.claude.com/docs/build-with-claude/token-counting) is free but needs an API key; both count a description's text but not what a harness wraps around it, so they serve only as a cross-check.
- Neither harness sends telemetry to a marketplace: the user or the organisation chooses the endpoint, and managed settings can enforce it.
  A project's `.claude/settings.json` cannot turn telemetry on or choose its endpoint, though it can turn a signal off.
- The OpenTelemetry generative AI conventions define `gen_ai.skill.*` attributes for loading a skill, on a `load_skill` span at `development` status, with no skill version ([semantic-conventions-genai](https://github.com/open-telemetry/semantic-conventions-genai), `model/gen-ai/spans.yaml` at commit `d28355d`).
- Hooks cannot replace telemetry for skills: neither harness has a hook event for a skill load, and a plugin hook that sends data home would act without the user's consent.

So telemetry cannot rebuild a user's task, and none of it reaches CECH; cases stay synthetic, as ADR 25 has them.
What telemetry can give CECH is measurement of its own runs: with OpenTelemetry turned on in each run, `cech eval` can record tokens, cost, time, retries and the skill loads of each arm.

## How many runs a result needs

- Runs of one case are correlated, so 20 cases run 5 times are not 100 independent samples.
  For 68 against 92 passes out of 100, the 95% interval of the difference is about 13 to 35 points if the runs were independent, and about 5 to 44 points with a correlation of 0.6 within a case; error bars should be clustered by case ([Adding Error Bars to Evals](https://arxiv.org/abs/2411.00640)).
- About 43 runs per arm detect 68% against 92% with 80% power; a 10-point gap, 80% against 90%, needs about 200 per arm.
- These figures are computed here: Newcombe and Wilson intervals, a design effect of 1 + (m − 1)ρ with m = 5 runs per case, and a two-proportion power test at a two-sided α of 0.05.
- At three runs per arm, 2 of 3 passes has a 95% Wilson interval of about 21% to 94%, so three runs per arm detect only gross failures.
- pass@k, at least one pass in k runs, and pass^k, every one of k runs passing, measure different things; a skill that users rely on is judged by pass^k ([Demystifying evals for AI agents](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents)).
- Anthropic's skill-creator compares an existing skill with its previous version as the baseline, and reports pass rate, time and tokens with their spread ([skill-creator](https://github.com/anthropics/skills/blob/main/skills/skill-creator/SKILL.md)).
- A matrix of three arms (without the plugin, with it, and with the last release), 20 cases and 5 runs on two harnesses is 600 runs a release; Codex runs one at a time per login, as the evals brief records.

## Graders for files a skill writes

- `claude plugin eval`'s graders read text: a `regex` on `{source: file}` sees the raw bytes of a zipped format such as `.docx`.
- Most checks of a `.docx` can be made by a script: python-docx reads its paragraphs, tables and page sizes.
  Whether a table fits the page needs a render, such as `soffice --headless --convert-to pdf`, as Anthropic's docx skill does ([docx skill](https://github.com/anthropics/skills/blob/main/skills/docx/SKILL.md)).
- "Flags missing data instead of inventing it" can be checked without a model only when the fixture leaves known fields out on purpose.

## Eval tools

- Google's EvalBench, Apache-2.0, `google-evalbench` 1.21.0 of 2026-10-27 on [PyPI](https://pypi.org/project/google-evalbench/), drives Claude Code and Codex headless, with skill and trajectory scorers ([repository](https://github.com/GoogleCloudPlatform/evalbench)).
  A model plays the user.
  It has no built-in comparison of a run without the skill and with each version.
  It writes score tables to CSV or BigQuery, not recordings of each run that a grader can check again, as ADR 25 item 4 keeps, and adds no telemetry of its own.
  It does not change ADR 25's rejection of existing runners, but its scorer for which skills a run loaded is worth comparing with CECH's load detectors.
- `agent-skill-eval` on PyPI runs skills through the Claude Code, Codex and OpenCode CLIs with and without the skill; its maturity was not rechecked.
- OpenAI's Plugin Eval 0.1.2, read on 2026-11-20 in the ChatGPT plugin directory and at its [source](https://github.com/openai/plugins/tree/main/plugins/plugin-eval), is a Node.js CLI for Codex only, not published as a package and without release tags.
  Its manifest says MIT, but neither the repository nor the plugin folder holds a licence file.
  `analyze` scores a plugin and its skills offline: manifest fields, frontmatter keys, the length of `description`, a "use when" sentence, broken relative links, file sizes and the tokens a description adds to every session.
  `benchmark` runs scenarios through `codex exec` with shell-command checks, but has no arm without the plugin, no routing and no repeated runs, and keeps each run's log without grading it again; `compare` diffs two scores.
  mise cannot pin it to an exact version (ADR 5 item 8), and its checks are partly its authors' advice and partly Claude Code's frontmatter keys, none of them probed against Codex (ADR 9 item 3), so CECH does not use it.
  Its offline checks of a skill's structure suit `cech validate`, for both harnesses, each Codex rule backed by a probe; its shell checks resemble the script grader of ADR 25 item 4, but run live in the run's folder, not offline against recordings.
