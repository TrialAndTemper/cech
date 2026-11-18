# 26. The release check

- Status: accepted
- Date: 2026-11-18
- Deciders: Michał Bożek

## Context

[ADR 25](0025-evals.md) sets each plugin's cases and the runner; a merge never waits for a model.
A release needs evidence from runs with a model that a skill works and, if changed, has not got worse.
Such runs cost money and vary, so their number trades cost against the smallest regression they catch.

Computed on 2026-11-18, for a test that catches a real drop 4 times in 5 and raises a false alarm once in 20:

- with pass rates near 85%, a drop of 15 points takes about 70 runs per arm to catch, and a drop of 10 points about 160;
- a rule that stops working, such as a drop from 90% to 50%, takes about 13;
- a sequential probability ratio test, which stops once the result is clear, needs on average roughly half the runs of a fixed test with the same error rates.

Read on 2026-11-18: Anthropic's skill-creator compares a changed skill with its previous version, and both harnesses export token counts, durations and retry attempts through OpenTelemetry (Claude Code 2.1.324 docs, Codex CLI docs).

## Decision

1. **When.**
   The release check runs before a release that adds a plugin or changes its skill, on both harnesses, with the cheapest model each harness supports, since a skill must help that model too.
   It never blocks a merge.
2. **Arms.**
   Content and control cases run routed, with and without the plugin and, for a changed skill, with the plugin as the last release holds it.
   Trigger cases run with the plugin, routed and unrouted.
   A case's arms run together, on the same harness and model versions.
3. **Gates.**
   On each harness, the release passes when:
   - its critical content cases pass with the plugin;
   - the routed skill loads when it should, and seldom otherwise;
   - its control cases do not differ between the arms with and without the plugin;
   - for a changed skill, its critical content cases are no worse than at the last release.

   The first two read the runs made; the last two are comparisons.
4. **Comparisons need clear evidence.**
   A comparison passes only when the runs show that any difference is smaller than a threshold, and fails when they show a difference of at least the threshold; a sequential test adds runs until one is shown, up to a cap.
   A critical case that drops sharply in its first runs fails the release at once.
   The runner's spec sets the threshold, the confidence, the cap and a budget per release, from the cost of the first plugin's runs.
5. **No result never passes.**
   A comparison still unclear at the cap or the budget has no result, as does a run that errors; the release notes name it, and a person decides.
6. **The report**, with each run's cost from the harness's telemetry, goes in the run results, never in the recordings that ADR 25 item 4 keeps.
   The release notes name any regression smaller than the threshold and any rule that showed no effect; removing such a rule is a person's call.

## Consequences

- A release stops once each comparison is clear; one whose real change is about half the threshold runs longest.
- A regression smaller than the threshold can ship, and the report shows it.
- The first release with a plugin waits for the runner's spec to set the threshold, the confidence, the cap and the budget.

## Options considered

- **Three runs per arm:** one chance pass or fail moves a pass rate by 33 points, so it catches only gross failures.
- **A fixed number of runs:** a large one makes every release pay in full, and a small one that gates only large drops lets moderate regressions ship.
- **Failing only on a drop shown at the set confidence:** cheap, but a moderate drop passes unproven.
- **Running the last release without gating on it:** shows a regression, but lets it ship.
- **Gating the gain over the arm without the plugin:** a rule the model often follows anyway can still be worth keeping, which a person judges from the report.
- **Gating unrouted loading:** CECH projects route their skills, so users meet the routed rate.
