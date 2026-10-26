# 5. Native harness features first

- Status: accepted
- Date: 2026-10-16
- Deciders: Michał Bożek

## Context

Things that once needed scripts are now built into Claude Code and Codex: per-project plugin enablement, marketplaces declared in project settings, Codex reading Claude-shaped plugins, `claude plugin validate` and `claude plugin eval`.
Custom copies of such features can break and conflict with the harnesses' own handling.
Harness behaviour can also change between releases.

## Decision

1. Before writing CECH code, check whether a harness already does it.
   If so, use it and add only what is missing.
2. Each workaround for a harness gap is recorded once, in `data/harnesses.toml`, with the gap it covers and the check that shows when to delete it.
3. `data/harnesses.toml` lists each behaviour CECH relies on, the probe that confirms it, and the last verified harness version.
   It ships with the `cech` CLI, so it is product data, not dev-tool config.
   Docs or `--help` cited there carry date and version.
   A pull request that starts relying on a behaviour adds it.
4. A scheduled CI job runs the probes and the workaround checks on the newest releases.
   It opens an issue when a probe fails or a gap closes.
   The `cech` CLI warns when an installed harness is newer than the versions recorded there, but never blocks; the scheduled job catches breaks.

## Consequences

- Less CECH code to break.
- A workaround is deleted once its harness closes the gap.

## Options considered

- **A harness-independent layer over both harnesses:** more code, always behind.
- **Trust docs without probes:** docs can lag behind behaviour.
