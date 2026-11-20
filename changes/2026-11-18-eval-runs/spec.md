# Eval runs and the release check

`cech eval` runs cases with a model for the release check of [ADR 26](../../docs/adr/0026-the-release-check.md); [research.md](research.md) holds the findings behind these rules, and [the evals brief](../done/2026-10-29-evals/brief.md) the notes on running each harness.

## Acceptance criteria

### Runs

1. Each arm of a content or control case, and each trigger case routed and unrouted, starts at five runs.
   Test: to be written
2. While a comparison is unclear, five more runs are added to each arm it reads, up to the cap; an arm that no comparison reads stays at five.
   Test: to be written
3. A comparison is a sequential probability ratio test of "no difference" against "a difference of at least the threshold", in pass-rate points, at the confidence, and corrects for checking the result after every five runs.
   Test: to be written
4. A comparison that pools cases adds five runs to each case it pools, and is stratified by case.
   Test: to be written

### Gates

Each gate applies on each harness.

5. Each critical content case passes in more than half of its runs with the plugin, read from the runs made.
   Test: to be written
6. Routed, the skill loads in at least 70% of the trigger runs that should load it and at most 20% of the others, read from the runs made.
   Test: to be written
7. For each control case, the test shows no difference of at least the threshold between each arm with the plugin and the arm without it.
   Test: to be written
8. For a changed skill, the test shows its critical content cases, pooled, no worse than at the last release by the threshold.
   Test: to be written
9. A critical case that the last release passes in at least four of its first five runs, and the new version in at most one, fails the release at once.
   Test: to be written
10. A comparison still unclear at the cap, or when the release's budget runs out, has no result, and so does a run that errors; no result never passes.
    Test: to be written

### Report

11. The report gives, per harness and arm:
    - each gate's result, with the runs it read;
    - pass rates, with intervals clustered by case;
    - the unrouted load rates and the differences below the threshold;
    - each skill version's description size and the tokens its plugin adds (criterion 14);
    - the name of every case without a result.

    Test: to be written
12. Each run records its tokens, cost, time and retry attempts from the harness's OpenTelemetry, with skill and plugin names shown, in the run results under `evals/results/`, never in a recording.
    Test: to be written
13. A case's arms run in one release check, on the same harness and model versions, with each harness's cheapest supported model, and the report names those versions.
    No run loads a plugin or MCP server other than its arm's plugin, those the fixture project sets and the harness's built-ins, which are the same in every arm.
    Each run records the tools and plugins its harness loaded.
    Test: to be written
14. For each skill version the release check runs, on each harness, the report gives its description's size in Unicode code points and the input tokens its plugin adds to the first request of the main agent loop.
    For each content and control case, the k-th run of that version's arm is paired with the k-th run of the arm without the plugin, up to the smaller count; the figure is the median over all pairs of the difference in that request's whole input, as each harness counts it.
    A pair whose loaded tools or plugins differ by more than the plugin is left out of the median and named in the report.
    Test: to be written

## Out of scope

- How a run achieves the isolation of criterion 13 and how the plugin is installed in each arm, including from a release tag: the evals brief holds the notes.
- The script grader that ADR 25 item 4 allows; it comes with the first plugin that writes files a text grader cannot read.

## Open questions

- The threshold, the confidence, the cap of runs per case and the budget per release, set from the cost of the first plugin's runs.
- Where Codex reports the tools and plugins a run loaded, which criteria 13 and 14 need.
- How the release workflow reads the release check's result for the release commit, and where a person's decision on a release without a result is recorded.
- Whether the control-case tests need a correction for testing many cases at once, so that one false difference does not fail most releases.
