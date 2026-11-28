# 27. The release check

- Status: accepted
- Date: 2026-11-18
- Deciders: Michał Bożek

## Context

[ADR 25](0025-evals.md) sets each plugin's cases and the runner.
Before a release, model runs must show that a skill works and has not got worse.
Runs cost time and vary: more runs catch smaller drops but cost more.

Computed on 2026-11-18, for a test that misses a real drop once in 5 and raises a false alarm once in 20:

- for independent runs with pass rates near 85%, a drop of 15 points takes about 70 runs per arm to catch, and a drop of 10 points about 160;
- a claim that stops working, such as a drop from 90% to 50%, takes about 13 runs;
- a sequential test stops once the result is clear, so a large change needs fewer runs than a small one.

Read on 2026-11-18: Anthropic's skill-creator (`anthropics/skills` at commit `b0cbd3d`) runs a changed skill against its previous version.

## Decision

1. **Before each release that adds or changes a plugin.**
   The release check runs on each harness the skill supports ([ADR 10](0010-claude-code-and-codex-from-one-source.md) item 4), with that harness's cheapest model, since a skill must work even with it.
   It runs as [ADR 26](0026-eval-runs-in-containers.md) sets, never blocking a merge.
   The release workflow ([ADR 14](0014-releases-and-versions.md) item 6) pushes the catalogue tag only when the check passes for every added or changed plugin, or a person accepts a bar with no result.
   An extension may require it for its own releases.
2. **A third arm for a changed plugin.**
   Content and control cases run in the two routed arms of [ADR 25](0025-evals.md) item 1; for a changed plugin, content cases also run in a third, with the latest final release's plugin.
3. **Four bars on each harness.**
   On each harness, the release passes when:
   - its content cases pass with the plugin;
   - the routed skill loads when it should, and seldom otherwise;
   - its control cases do not differ between the arms with and without the plugin;
   - for a changed plugin, no content case it shares with the latest final release has dropped.
4. **Bars need clear evidence.**
   A bar passes only when the runs show it holds, and fails when they show it does not.
   A comparison of two arms holds when their difference is below a threshold.
   A sequential test adds runs until the bar passes or fails, up to a cap per case.
   The evals reference gives the threshold, the false-alarm and missed-drop rates, the cap and a budget of runs per release.
5. **No result never passes.**
   A bar unclear at the cap or the budget has no result, as does a case whose runs often error; the release notes name it, and a person decides whether to release.
6. **A committed report.**
   `cech eval` writes the report in one fixed form, with what the runs cost and no local paths, session ids or account data.
   It is committed before the release, and the release workflow checks that the report names the commit of `main` the runs used and that no plugin or eval folder changed since.
   The full run results are attached to the release, never kept as recordings ([ADR 25](0025-evals.md) item 4).

## Consequences

- Runs stop once each bar is clear; a drop close to the threshold needs the most runs.
- A drop smaller than the threshold can ship, and the report shows it.
- The threshold, cap and budget were set on a test plugin; real plugins may need others.

## Options considered

- **A fixed number of runs:** a large one makes every release pay in full, and a small one lets moderate drops ship; at three per arm, one chance result moves a pass rate by 33 points.
- **Failing only on a proven drop:** cheap, but moderate drops pass unproven.
- **One comparison pooling a plugin's content cases:** a gain in one case can hide a large drop in another.
- **A third arm without a bar:** shows a drop, but lets it ship.
- **A bar on the gain over the arm without the plugin:** a claim the model often meets anyway may still be worth keeping.
- **A bar only on cases marked critical:** a skill with none marked could get worse unchecked, and every claim must hold ([ADR 10](0010-claude-code-and-codex-from-one-source.md) item 4).
- **The report only in the run results:** no workflow can read it, and nobody reviews it.
- **Every run's results in git:** hundreds of files per release.
- **The harness's default model:** costs more per run, and a skill shown to work on a stronger model may still fail on the cheapest.
- **A bar on unrouted loading:** CECH projects route their skills, so users meet the routed rate.
