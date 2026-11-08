# Reference

These pages describe what CECH does now, for the files, commands and checks that users, scripts and extensions rely on ([ADR 24](../adr/0024-where-each-kind-of-text-lives.md) item 2).
They cover only what exists: behaviour that is not built yet is in its change's folder under `changes/`, not here.
The reason behind each choice is in the [ADRs](../adr/README.md), and how to contribute is in [CONTRIBUTING.md](../../CONTRIBUTING.md).

A change to behaviour that a page describes updates the page in the same commit.
Where a page and the code disagree, the code and its tests are right, and the page is fixed.

## Pages

- [A marketplace](marketplace.md): its layout, name rules, categories and release builds.
- [Generated files](generated-files.md): what `cech build` writes, and from what.
- [The harness data file](harness-data.md): its entries, their keys, and how its probes are run.
- [The term check](term-check.md): the `terms` gate's tiers, what it scans, its exceptions and its output.
