# Reference

These pages describe what CECH does now, for the files, commands and checks that users, scripts and extensions rely on ([ADR 2](../adr/0002-where-each-kind-of-text-lives.md) item 2).
They cover only what exists: behaviour that is not built yet is in its change's folder under `changes/`, not here.
The reason behind each choice is in the [ADRs](../adr/README.md), and how to contribute is in [CONTRIBUTING.md](../../CONTRIBUTING.md).

A change to behaviour that a page describes updates the page in the same commit.
Where a page and the code disagree, the code and its tests are right, and the page is fixed.

## Pages

- [The `cech` CLI](cli.md): its commands, flags, output and exit statuses.
- [A marketplace](marketplace.md): its data file, layout, name rules, categories, release builds, advice for a private extension, and how to measure its skills in a harness's telemetry.
- [Evals](evals.md): a plugin's eval folder, its cases and recordings, what `cech eval --check` checks, and the run results, report and values of the release check.
- [Generated files](generated-files.md): what `cech build` writes, and from what.
- [The harness data file](harness-data.md): its entries, their keys, and how its probes are run.
- [The term check](term-check.md): the `terms` gate's tiers, what it scans, its exceptions and its output.
- [The approval diff comment](approval-diff.md): when an approval holds, the diff comment that shows what changed since a dismissed one, and its workflows.
