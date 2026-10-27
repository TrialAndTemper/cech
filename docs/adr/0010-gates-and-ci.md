# 10. Gates and CI

- Status: accepted
- Date: 2026-10-19
- Deciders: Michał Bożek

## Context

Earlier ADRs promise checks: ADR numbers ([ADR 1](0001-record-architecture-decisions.md)), DCO sign-off ([ADR 2](0002-licence-and-contributions.md)), the term check ([ADR 4](0004-vendor-neutrality.md)), the scheduled probes ([ADR 5](0005-native-tool-features-first.md)), the validator ([ADR 6](0006-claude-code-and-codex-from-one-source.md), [ADR 7](0007-one-plugin-one-skill.md), [ADR 8](0008-categories-and-names.md)) and the build check ([ADR 9](0009-generated-files.md)).
A check that runs only on some machines, or differently on each, lets breakage through.
Git hooks can be skipped and do not run on edits made in the GitHub web interface.

[GitHub docs](https://docs.github.com/en/billing/managing-billing-for-your-products/managing-billing-for-github-actions/about-billing-for-github-actions), read on 2026-10-17: standard GitHub-hosted runners are free in public repositories; private repositories on the Free plan get 2,000 minutes a month; per-minute prices are higher for Windows and macOS runners than for Linux.

Check tools give different results in different versions, so contributors and CI need the same versions.
A gate that breaks on `main` blocks every merge, including the merge that would fix it.

mise installs the tools pinned in `mise.toml` and runs its tasks; probed with mise 2026.10.3 on 2026-10-19:

- `mise run` puts each pinned tool on `PATH` without shell activation;
- `mise lock` records each tool's download and checksum per platform; bats has no Windows build;
- with `locked` set, an install fails on an unlocked version or a checksum mismatch;
- `minimum_release_age` applies when mise resolves a version, not to an exact version given by hand;
- mise runs a `mise.toml` only after `mise trust` in each clone, and with `paranoid` on asks again after each change;
- a tool's `os` option limits it to listed systems, which `mise lock` follows;
- on Windows a task runs with cmd unless `run_windows` gives another command.

Workflows run third-party actions with the repository's token, and anyone can open a pull request from a fork.
A mutable action tag or a workflow that hands secrets to fork code can leak secrets or alter the repository.

## Decision

1. **One entry point: mise.**
   mise is the required task runner.
   `mise run check` runs every gate; `mise run <gate>` runs one.
   Each gate is a task in `mise.toml` that runs a pinned tool, CECH's CLI, or a POSIX shell script under `scripts/`, so contributors and CI run one implementation with the same pinned tools.
   On Windows a script task runs through Git Bash's `sh`, and bats, limited to Linux and macOS by `os`, does not run.
2. **CI is the authority.**
   A GitHub Actions workflow runs `mise run check`, the command a contributor runs, on every pull request and push to `main`, so it holds no second list of gates.
   The gates, the link check included, run on Linux, macOS and Windows, apart from the bats tests on Windows, and each keeps running after another fails.
   A ruleset on `main` requires the gates to pass before merge.
   Gates that need a secret, such as the private term tier ([ADR 4](0004-vendor-neutrality.md)), cannot run on fork pull requests, so they are required only where they run and are skipped locally without the secret.
   Only the private term tier has a step of its own, so that its secret reaches no other gate.
   A rule that CI cannot check is written as guidance in the contributing guide, not described as enforced.
3. **Bypass.**
   Organisation owners may bypass the ruleset only to repair a broken `main` or a broken gate.
   Each bypass gets an issue saying what was done and why.
4. **Hooks are a convenience.**
   A `pre-commit` hook (term check, lint, format checks and workflow lint on the staged tree) and a `commit-msg` hook (term check, message format, and a sign-off checked against the author git will record, as CI does) are offered for local use.
   They call `mise run` with the gate's staged or message mode.
   No rule exists only in a hook.
5. **The gates**, each added when the thing it checks first exists:
   - term check, all public tiers; private tier as in [ADR 4](0004-vendor-neutrality.md);
   - DCO sign-off on every commit in the pull request;
   - commit message format;
   - Markdown lint and link check;
   - shell lint and format check of the gate scripts;
   - tests of the gate scripts;
   - workflow lint;
   - ADR numbering and index;
   - `cech build --check` and `cech validate`;
   - eval case lint and grader tests (a later ADR on evals);
   - CLI tests on Linux, macOS and Windows.

   A gate script has a test for each way the gate could pass when it should fail, where review would likely miss it, such as a finding it loses or a check it skips.
   Such a way mostly hides outside the diff: in a bumped tool pin, on a platform the reviewer does not run, or in a gate run indirectly, such as on the staged tree.
   Each case added since 2026-10-26 says in a comment which way it guards.
   Any other way is checked by hand when the gate is added or changed, and the pull request records that check.
   A gate that fails when it should pass shows itself the first time it runs on that input, so that case needs no test.
6. **Tool versions are pinned exactly** in the `[tools]` table of `mise.toml`, with downloads and checksums in `mise.lock`.
   CI installs an exact mise version, then the tools with `locked` set; `mise.toml` sets mise's `min_version`.
   A tool that mise builds from source with its `go` backend, such as govulncheck, has no download to record: `mise.lock` holds only its version, and the Go checksum database checks the source that `go install` fetches.
   Pins are added and bumped by letting mise resolve them under a week's `minimum_release_age`; review checks the release date of a version given by hand.
   The table holds only tools that build or check CECH; `data/tools.toml` ([ADR 5](0005-native-tool-features-first.md)) records the agent tools CECH relies on.
7. **Workflow safety.**
   Third-party actions are pinned by full commit SHA.
   Workflows default to `permissions: contents: read`, widened per job only where needed.
   Pull requests from forks run on `pull_request`, never `pull_request_target`, and never receive secrets.
   Jobs that need secrets (private terms, evals) run only on branches in the repository.
8. **Scheduled jobs are not merge gates.**
   The probes of [ADR 5](0005-native-tool-features-first.md) run on a schedule, open an issue on failure, and never block a pull request.

## Consequences

- A contributor installs mise, runs `mise trust` and `mise install`, then runs exactly what CI runs with one command.
- While the repository is private, CI spends the Free plan's minutes, which cost most on macOS and Windows.
  If a month's use passes half the allowance before going public, those jobs move to pushes to `main` and the schedule.
- A fork pull request can merge without the gates that need a secret; their run on `main` catches a problem after merge.

## Options considered

- **Hooks as the gate:** skipped with `--no-verify`, absent on web edits.
- **A shell script as the entry point, with mise optional:** contributors without mise install each pinned tool by hand, and two ways to run the gates can drift apart.
- **Another task runner (make, just):** it would still need a second tool to install the pins.
- **Version ranges, with exact versions only in the lockfile:** a lock refresh could change a tool's version with no visible change to its pin.
- **CI steps written straight into workflow files:** local runs drift from CI.
