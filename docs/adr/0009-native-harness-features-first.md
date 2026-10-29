# 9. Native harness features first

- Status: accepted
- Date: 2026-10-16
- Deciders: Michał Bożek

## Context

Things that once needed scripts are now built into Claude Code and Codex: per-project plugin enablement, marketplaces declared in project settings, Codex reading Claude-shaped plugins, `claude plugin validate` and `claude plugin eval`.
Custom copies of such features can break and conflict with the harnesses' own handling.
Harness behaviour can also change between releases.
A workaround is easier to delete when its code names the record of the gap it covers.
A typo in a hand-written data file should fail, not be ignored.
A probe that drives only a harness's plugin and debug commands, such as installing a plugin and reading the prompt it builds, calls no model and needs no login or API key (checked on 2026-10-29 with Claude Code 2.1.312 and Codex 0.165.0).
`go:embed` builds in only files in the package's own folder or below, and `go test` leaves files outside the module out of the key of its result cache (checked on 2026-10-29 with Go 1.27.3).

## Decision

1. Before writing CECH code, check whether a harness already does it.
   If so, use it and add only what is missing.
2. Each workaround for a harness gap is recorded once, in the harness data file `cli/internal/harnessdata/harnesses.toml`, with the gap it covers and a probe, a repeatable check of the harness, that shows when to delete it.
3. The file also lists each behaviour CECH relies on, the probe that confirms it, and the last verified harness version.
   The `cech` CLI builds it in, so it is product data, not dev-tool config, and a marketplace that extends CECH needs no copy of it.
   Docs or `--help` cited there carry date and version.
   A pull request that starts relying on a behaviour adds it.
4. On the newest harness releases, a scheduled CI job runs each probe that can run unattended, and a person runs the others.
   A result that differs from the one expected opens an issue: a behaviour changed or a gap closed.
   The `cech` CLI warns when an installed harness is newer than the versions recorded there, but never blocks; the scheduled job catches breaks.
5. **The format of the harness data file.**
   Each entry is a TOML table named `<harness>-<short-name>`, so TOML itself rejects two entries with one name.
   Code that works around a gap cites that name.
   An entry is a gap, which also says what CECH does instead, or a behaviour.
   Each gives its probe, the result expected while it holds, and the last harness version that gave that result, and may list the docs or `--help` it rests on.
   An entry whose probe can run unattended says so, and `scripts/probes/<entry name>` then carries out its probe and tells the expected result from another and from a probe that could not run; the scheduled job runs those scripts.
   Reading the file fails on an unknown, missing or ill-formed key, and names the entry and the key of every such problem.
   A test of the CLI holds the committed file to these rules, so the merge gate fails on a wrong entry.

## Consequences

- Less CECH code to break.
- A workaround is deleted once its harness closes the gap.
- A probe that cannot run unattended needs a person to run it, and is only as exact as its prose.
- A probe that can has both prose and a script, which a change to one must keep in step with the other.

## Options considered

- **A harness-independent layer over both harnesses:** more code, always behind.
- **Trust docs without probes:** docs can lag behind behaviour.
- **The file in `data/`, beside other repository data:** the CLI could not build it in without a copy step that can drift, and a local test run could report a cached pass after the file changed.
- **Entries in an array, each with a `name` key:** TOML would allow two entries with one name, so the code would have to check it.
- **Every probe a runnable script:** a probe that needs a logged-in harness cannot run in the merge gate or an unattended job, and steps in prose are easier for a person to follow.
- **Probe scripts beside the data file in `cli/`:** a Go test could check that each exists, but shell scripts would sit in a Go package, apart from the other scripts and their lint.
