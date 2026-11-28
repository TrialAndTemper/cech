# Eval runs and the release check

Implemented 2026-11-27; not maintained: the code and its tests are current.

`cech eval` runs a marketplace's eval cases with a model, each run in its own container as [ADR 26](../../../docs/adr/0026-eval-runs-in-containers.md) sets, for the release check of [ADR 27](../../../docs/adr/0027-the-release-check.md).
[research.md](research.md) holds the findings behind these rules, and [the evals brief](../2026-10-29-evals/brief.md) the notes on running each harness.
The command's flags, refusals and exit statuses are in the section `cech eval` of [the CLI reference](../../../docs/reference/cli.md), and the values the rules name, such as the bars, the threshold, the cap and the budget, in the section Run results and reports of [Evals](../../../docs/reference/evals.md).

## Acceptance criteria

### Arms and rounds

1. A content or control case runs in two arms, with the plugin and without it, and a trigger case runs routed and unrouted.
   A content case of a changed plugin (criterion 22) that the latest final release also had runs in a third arm, `latest-release`, with that release's plugin.
   Each arm starts at five runs.
   Test: `cli/internal/app/evalrun_test.go` `TestEvalDryRun`, `TestEvalComparesAChangedPlugin`; `cli/internal/releasecheck/check_test.go` `TestArmsAndBars`; `cli/internal/releaserun/releaserun_test.go` `TestRunBuildsEachArm`
2. WHEN a bar is unclear, THEN each arm it reads gets five more runs, up to the cap.
   While the budget lasts, the unclear bar whose arms have the fewest runs goes first.
   Among equals, the bar whose plugin and case sort first goes first, and a plugin's bar over its trigger cases goes before its other bars.
   Test: `cli/internal/evalstats/evalstats_test.go` `TestNext`; `cli/internal/releasecheck/check_test.go` `TestCheckGoodSkill`
3. A bar reads a confidence sequence: a range for a pass rate, or for the difference of two, that holds at the false-alarm rate however often it is read.
   It is the predictable plug-in empirical-Bernstein sequence for outcomes from 0 to 1 (research, How many runs a result needs); a difference pairs the k-th runs of its two arms, and maps their difference, from −1 to 1, onto 0 to 1.
   Test: `cli/internal/evalstats/evalstats_test.go` `TestRateIntervalHolds`, `TestDiffIntervalHolds`, `TestDiffPairsInOrder`, `TestIntervalsNarrow`, `TestNoRuns`
4. A bar passes when its whole range meets the bar, and fails when its whole range misses it; each case is read at the full false-alarm rate, with no correction for the number of cases.
   IF a bar is still unclear at the cap, or when the budget runs out, THEN it has no result, and no result never passes.
   Test: `cli/internal/evalstats/evalstats_test.go` `TestVerdicts`, `TestSettle`; `cli/internal/releasecheck/check_test.go` `TestCheckBudget`

### Bars

The bars apply on each harness the plugin supports (ADR 10 item 4), which is both until a plugin can say otherwise.

5. With the plugin, each content case's pass rate is above the content bar.
   Test: `cli/internal/evalstats/evalstats_test.go` `TestVerdicts`; `cli/internal/releasecheck/check_test.go` `TestCheckGoodSkill`, `TestCheckBrokenSkill`, `TestArmsAndBars`
6. In the routed arm, the skill's load rate is above the load bar over the trigger cases that should load it; over those that should not, its rate of wrong loads is below the skip bar.
   Test: `cli/internal/evalstats/evalstats_test.go` `TestVerdicts`; `cli/internal/releasecheck/check_test.go` `TestCheckGoodSkill`, `TestArmsAndBars`
7. For each control case, the pass rates with and without the plugin differ by less than the threshold, either way.
   Test: `cli/internal/evalstats/evalstats_test.go` `TestDiffIntervalHolds`, `TestVerdicts`; `cli/internal/releasecheck/check_test.go` `TestCheckGoodSkill`, `TestArmsAndBars`
8. For each content case in the `latest-release` arm, the pass rate with the plugin being released is lower than with the release's plugin by less than the threshold, if at all.
   Test: `cli/internal/evalstats/evalstats_test.go` `TestDiffIntervalHolds`, `TestVerdicts`; `cli/internal/releasecheck/check_test.go` `TestCheckGoodSkill`, `TestCheckBrokenSkill`, `TestArmsAndBars`

### Runs that error

9. IF a run errors, or runs longer than the run timeout, THEN it is run once more, and the rerun takes its place for good; the rerun counts against the budget, not the cap.
   A run that criterion 12 counts as erroring is not run again.
   Test: `cli/internal/evalrun/run_test.go` `TestExecuteCutOff`, `TestExecuteTimeout`; `cli/internal/evals/live_test.go` `TestTrimNeedsTheLastLine`; `cli/internal/releasecheck/check_test.go` `TestCheckErrors`, `TestCheckRerunCountsAgainstTheBudget`; `cli/internal/releaserun/releaserun_test.go` `TestRunErrors`, `TestRunResumesARerun`
10. IF more than the error share of a case's runs still error, checked from its 20th run on and at the end, THEN the case has no result.
    Every bar that reads it has no result too, and those bars' arms make no more runs.
    Otherwise a case's rates count only the runs with a result.
    Test: `cli/internal/evalstats/evalstats_test.go` `TestTooManyErrors`; `cli/internal/releasecheck/check_test.go` `TestCheckErrors`, `TestCheckTriggerErrors`

### Grading a run

11. A run is graded by the same code as a recording under `cech eval --check`: its trace is trimmed first, a content or control run passes when every grader passes, and a trigger run passes when its harness's load detector reads a load.
    Its created files are the paths outside `.git/` that are new or changed between the workspace with the staged change in place and the workspace after the run; a deleted path is not a created file.
    Test: `cli/internal/evalrun/run_test.go` `TestExecuteClaude`, `TestBuildWorkspace`, `TestCreatedFiles`; `cli/internal/evals/live_test.go` `TestTrimMakesARecording`, `TestGradeRun`, `TestGradeRunResultFiles`, `TestGradeRunBadCase`; `cli/internal/releaserun/releaserun_test.go` `TestRunGradesAndKeeps`
12. Each run records the plugins, skills and MCP servers its harness loaded; on Codex, whose `codex exec --json` names none, `cech eval` reads them with `codex debug prompt-input` and `codex mcp list` in the run's container before the run.
    On each harness, the runs without the plugin set the baseline: everything any of them loaded.
    IF a run with the plugin loaded anything outside the baseline, apart from its own arm's plugins and their skills, THEN it counts as a run that errors; it is judged against the baseline at each read, so its verdict can change as the baseline grows.
    Test: `cli/internal/evalrun/run_test.go` `TestExecuteClaude`, `TestExecuteCodexLogin`; `cli/internal/releasecheck/check_test.go` `TestCheckBaseline`, `TestCheckBaselineGrows`
13. `cech eval --check` reports `critical` in a case and `weight` or `arm` in a grader as keys it does not take, and checks `evals/reports/` as [Evals](../../../docs/reference/evals.md) gives; it is otherwise unchanged.
    Test: `cli/internal/app/eval_test.go` `TestEvalCheckFindings`; `cli/internal/evals/check_test.go` `TestCheckReportsCategory`

### Where runs happen

14. Each run starts in a fresh container from the eval image, under Podman or Docker, with only the environment `cech eval` sets.
    It mounts its workspace and its home.
    It also mounts, read-only, the marketplace its arm installs from and, on Claude Code, the folder of criterion 19, and nothing else.
    Test: `cli/internal/evalrun/run_test.go` `TestExecuteClaude`
15. A run's only network is an internal one, shared with two containers from the same image: a proxy that lets through only its harness's hosts, and a forwarder to the telemetry receiver of criterion 24.
    The forwarder runs only socat, reaches no other network, and is the only container without its SELinux label (research, Where runs happen and on whose account).
    Test: `cli/internal/evalrun/run_test.go` `TestExecuteClaude`, `TestFilter`, `TestStartBatch`
16. Every container `cech eval` starts has no capabilities, cannot gain privileges, has limits on its processes and memory, and carries its batch's label and name.
    Test: `cli/internal/evalrun/run_test.go` `TestStartBatch`
17. A case's arms run in one batch, as ADR 26 item 2 defines it, with the cheapest model of each harness.
    Test: `cli/internal/evalrun/run_test.go` `TestExecuteClaude`; `cli/internal/releaserun/releaserun_test.go` `TestRunBuildsEachArm`
18. A workflow builds the image from `images/eval/Containerfile`, with each base image pinned by digest and each harness at an exact version with its checksum, the newest release at least a week old (ADR 5 item 10), and publishes it to the GitHub container registry as a public package.
    It runs when started by hand and on a push to `main` that changes `images/eval/`, and only its publishing job may write packages.
    Test: by hand on 2026-11-27: `podman build --file images/eval/Containerfile images/eval` built the image from the pinned bases and checksums, `mise run lint:actions` passes, and the workflow runs on `workflow_dispatch` and on a push to `main` under `images/eval/`, with `packages: write` only on its publish job; its first publish waits for that push.
19. The image holds Codex and the pinned lockfile of Claude Code, but not Claude Code itself.
    `cech eval` installs Claude Code from that lockfile once per image, into a folder in the user's cache.
    The install runs in a container from the image, on the engine's default network, with no credential, and writes a marker last.
    IF that folder has no marker, THEN it is emptied and installed again.
    Test: `cli/internal/evalrun/run_test.go` `TestInstallClaudeCode`; by hand on 2026-11-27: in a container from the image, `codex --version` gives 0.171.0, `command -v claude` finds nothing, and `/cech/claude-code/package-lock.json` is there.

### Credentials

20. Each run gets one credential, the first that the CLI reference lists for its harness, from the environment or the system keyring.
    No credential appears in a trace, the run results, the output or the report file.
    Test: `cli/internal/app/evalrun_test.go` `TestCredential`; `cli/internal/evalrun/keyring_test.go` `TestFromKeyring`; `cli/internal/evalrun/run_test.go` `TestExecuteClaude`; by hand on 2026-11-27 ([research](research.md), The hand checks): no credential in the run results, the report or the command's output.
21. WHEN the credential is a ChatGPT login, THEN `cech eval` runs `codex login status` on a copy of the login, in a container from the image, and puts the copy back in place of the login if Codex changed it.
    Each run gets its own copy in its home, never written back, and no container mounts the user's Codex home.
    Test: `cli/internal/evalrun/run_test.go` `TestExecuteCodexLogin`, `TestRefreshCodexLogin`

### Changed plugins

22. A plugin has changed when `git diff` between the commit of `main` that the latest final release was cut from and `HEAD` shows a change under its folder, `plugins/<category>/<plugin>/`.
    A change that `cech build` writes into every plugin folder, such as one from `data/marketplace.toml` or the root `LICENSE`, changes every plugin, and a plugin changes with a plugin it depends on.
    A plugin that no final release holds is new.
    Test: `cli/internal/evalchange/evalchange_test.go` `TestChanged`, `TestChangedWithoutFinalRelease`
23. An arm with the plugin installs it and the plugins it depends on, directly or through others, from `HEAD`'s tree; the `latest-release` arm installs them from the release's tag, and only those the tag holds.
    Every arm takes the case, its fixture and its graders from `HEAD`.
    Test: `cli/internal/app/evalrun_test.go` `TestStage`; `cli/internal/evalchange/evalchange_test.go` `TestNeeds`, `TestExportRelease`, `TestExportReleaseNamesMissingPlugins`; `cli/internal/evalrun/run_test.go` `TestExecuteClaude`; `cli/internal/releaserun/releaserun_test.go` `TestRunBuildsEachArm`

### Telemetry

24. Each run's telemetry comes from its harness's OpenTelemetry, which `cech eval` receives itself over OTLP/HTTP, the OpenTelemetry protocol, with protobuf and decodes with the OpenTelemetry project's Go modules.
    A run keeps only the fields that [Evals](../../../docs/reference/evals.md) lists for `telemetry.json`, in its run results and never in a recording.
    IF a run's telemetry cannot be read, THEN it counts as a run that errors.
    Test: `cli/internal/otlpsink/otlpsink_test.go` `TestClaudeCode`, `TestCodex`, `TestRunsApart`, `TestRejected`; `cli/internal/releaserun/releaserun_test.go` `TestRunGradesAndKeeps`, `TestRunErrors`
25. The report gives, on each harness, each skill version's description size in Unicode code points and the input tokens its plugin adds to the first request of the main agent loop.
    The versions are this commit's and, for compared cases, the latest final release's.
    The figure is the median, over the content and control cases, of the difference in that request's whole input between the k-th kept runs of the version's arm and of the arm without the plugin.
    A pair whose runs differ in what they loaded by more than the plugin's own plugins and skills is left out and named.
    Test: `cli/internal/otlpsink/otlpsink_test.go` `TestClaudeCode`, `TestCodex`; `cli/internal/releaserun/releaserun_test.go` `TestCosts`, `TestCostsOfTheLatestRelease`

### Run results and the report

26. The run results go where [Evals](../../../docs/reference/evals.md) gives, each run with its trace trimmed and scrubbed as recordings are, so they can be attached to a release.
    Test: `cli/internal/evalrun/run_test.go` `TestExecuteClaude`; `cli/internal/evals/live_test.go` `TestTrimMakesARecording`, `TestScrub`
27. `cech eval` writes the report file `evals/reports/<commit>.md` from a template built into the CLI, with what [Evals](../../../docs/reference/evals.md) lists, the runs' tokens and cost among it.
    The file passes `mise run lint:markdown` and holds no local path, which `cech eval --check` checks; its template has no field for a session id or account data.
    Test: `cli/internal/app/evalrun_test.go` `TestEvalDryRun`; `cli/internal/app/eval_test.go` `TestEvalCheckPasses`, `TestEvalCheckFindings`; `cli/internal/releasecheck/check_test.go` `TestCheckGraders`; `cli/internal/releasecheck/report_test.go` `TestRender`; `cli/internal/releaserun/releaserun_test.go` `TestRunFailedGraders`

### The command

28. `cech eval` runs the release check at `HEAD` with the flags and output that the CLI reference gives.
    It stages the marketplace and the fixture projects from `HEAD`'s tree, so nothing uncommitted or ignored reaches a run.
    It reads no variable that marks CI.
    Test: `cli/internal/app/evalrun_test.go` `TestEvalDryRun`, `TestEvalUnknownPlugin`, `TestEvalUnknownPluginInADirtyTree`, `TestStage`; `cli/internal/evalchange/evalchange_test.go` `TestExportTree`; `cli/internal/evalrun/run_test.go` `TestBuildWorkspace`; by hand on 2026-11-27: `git grep -nE "Getenv|LookupEnv|Environ"` over the CLI's code finds it reading only `CECH_EVAL_IMAGE`, the credential variables and `CODEX_HOME`, and handing its environment on to the engine and to git.
29. IF `cech eval` refuses to run, or a harness has no result for a reason other than its bars, THEN it says why and exits as the CLI reference gives.
    It exits 1 for a tree that differs from `HEAD`, a problem in the eval cases, a plugin that depends on one the marketplace does not hold, a harness with no credential that `--harness` does not leave out, or a runner that cannot work.
    It exits 3 for a read denied while checking the cases, missing release tags, no eval image, no Podman or Docker, an image it cannot pull, another `cech eval` on the same commit's results, or a harness whose hosts were not reached while every other harness passed.
    Test: `cli/internal/app/evalrun_test.go` `TestEvalRefusesADirtyTree`, `TestEvalRefusals`, `TestEvalDeniedRead`, `TestEvalShallowClone`, `TestEvalNoImage`, `TestEvalNoEngine`, `TestEvalProxyFails`, `TestNoResult`, `TestExitFor`; `cli/internal/evalchange/evalchange_test.go` `TestChangedRefusesBadDependencies`, `TestLatestFinalInShallowClone`; `cli/internal/evalrun/lock_test.go` `TestLock`; `cli/internal/evalrun/run_test.go` `TestImageID`; `cli/internal/releasecheck/check_test.go` `TestCheckAbort`; `cli/internal/releaserun/releaserun_test.go` `TestRunAborts`; by hand on 2026-11-27 ([research](research.md), The hand checks): `--harness` leaves a harness with no credential out, and an image that cannot be pulled exits 3.
30. WHEN `cech eval` is interrupted, THEN it removes its containers and networks and the folders of the runs it stopped, keeps its finished runs, writes no report and exits 1.
    WHEN it is interrupted again, THEN it ends at once.
    Test: `cli/internal/evalrun/run_test.go` `TestCloseRemovesTheBatch`; `cli/internal/releaserun/releaserun_test.go` `TestRunInterrupted`; by hand on 2026-11-27 ([research](research.md), The hand checks): a single interrupt, and a second one soon after the first.
31. WHEN a later `cech eval` finds a finished run with the same key (the same commit, image ID, harness, model and `cech` versions, and kind of credential), THEN it reuses the run in its batch and baseline and against its budget, keeps a run that errored and its rerun in their places, and rewrites the report file from all the runs it reused and made.
    Test: `cli/internal/releaserun/releaserun_test.go` `TestRunGradesAndKeeps`, `TestRunErrors`, `TestRunResumesARerun`
32. Each harness behaviour the runner relies on has an entry and a probe in the harness data file ([ADR 9](../../../docs/adr/0009-native-harness-features-first.md) item 3), and the code that relies on it names the entry: at least the telemetry events and fields of criteria 24 and 25, and where each harness reports what it loaded.
    Test: `cli/internal/otlpsink/otlpsink_test.go` `TestCodexCapture`, `TestClaudeCodeCapture`; `cli/internal/harnessdata/harnessdata_test.go` `TestFile`
33. The CLI holds the values that [Evals](../../../docs/reference/evals.md) lists, the same for every marketplace.
    Test: by hand on 2026-11-27: `releaseStats` and `releaseBudget` in `cli/internal/app/evalrun.go` and `Defaults` in `cli/internal/evalrun/settings.go` hold the values of the table in [Evals](../../../docs/reference/evals.md), and no flag, variable or file of a marketplace changes them.

## Out of scope

- Values an extension sets for itself.
- The release workflow ([ADR 14](../../../docs/adr/0014-releases-and-versions.md) item 6, ADR 27 items 1, 5 and 6), a change of its own: it checks that the committed report names a commit of `main` and that nothing it covers changed since, records a person's acceptance of a bar with no result, says so in the release notes, and attaches the run results to the release.
- The script grader that ADR 25 item 4 allows; it comes with the first plugin that writes files a text grader cannot read.
- Installing the plugins a fixture project binds to roles, whose binding has no format yet, and, in an extension, the CECH plugins its plugins depend on, fetched from CECH's release; each comes with the change that gives it a format or a source.
- Checking that the values of research decision 28 hold on CECH's first real plugins, which comes with those plugins.
- A probe of whether `codex login status` refreshes a ChatGPT login old enough to need it; until one shows it, the harness data file has no entry for it.
