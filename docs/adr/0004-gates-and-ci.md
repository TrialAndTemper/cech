# 4. Gates and CI

- Status: accepted
- Date: 2026-10-19
- Deciders: Michał Bożek

## Context

ADRs promise checks, such as ADR numbering ([ADR 1](0001-record-architecture-decisions.md)) and the term check ([ADR 3](0003-vendor-neutrality.md)).
A check that runs only on some machines, or differently on each, lets breakage through.
Git hooks can be skipped, and do not run on web edits.
A broken gate on `main` blocks every merge, even the one that would fix it.

Probed with mise 2026.10.3 on 2026-10-19:

- `mise run` runs a task with the tools pinned in `mise.toml` on `PATH`, without shell activation;
- mise runs a `mise.toml` only after `mise trust`, and with `paranoid` on asks again after each change;
- on Windows a task runs with cmd unless it names another shell.

Workflows run third-party actions with the repository's token, and anyone can open a pull request from a fork.
A movable action tag, or a secret given to fork code, can leak secrets or change the repository.

## Decision

1. **One entry point: mise.**
   mise is the required task runner.
   `mise run check` runs every gate; `mise run <gate>` runs one.
   Each gate is a task in `mise.toml` that runs a pinned tool, CECH's CLI, or a POSIX shell script under `scripts/`, so contributors and CI run the same thing.
   On Windows a script task runs through Git Bash's `sh`, and bats, which has no Windows build, does not run.
2. **CI is the authority.**
   A GitHub Actions workflow runs `mise run check` on every pull request and push to `main`, on Linux, macOS and Windows, and holds no second list of gates.
   A ruleset on `main` requires the gates to pass before merge.
   A gate that needs a secret, such as the private term tier ([ADR 3](0003-vendor-neutrality.md)), is required only where the secret reaches, never on fork pull requests, and is skipped locally without it.
   A rule CI cannot check is guidance in the contributing guide, not described as enforced.
3. **Bypass.**
   Organisation owners may bypass the ruleset only to repair `main` or a gate.
   Each bypass gets an issue saying what was done and why.
4. **Hooks are a convenience.**
   Optional `pre-commit` and `commit-msg` hooks run some of the gates through `mise run`, on the staged tree or on the message; no rule exists only in a hook.
5. **The gates**, each added when the thing it checks first exists:
   - term check, all public tiers; private tier as in [ADR 3](0003-vendor-neutrality.md);
   - DCO sign-off on every commit in the pull request;
   - commit message format;
   - Markdown lint and link check;
   - shell lint and format check of the gate scripts;
   - tests of the gate scripts;
   - workflow lint;
   - ADR numbering, index and length;
   - `cech build --check` and `cech validate`;
   - code owners errors that GitHub reports;
   - eval case lint and grader tests;
   - CLI tests on Linux, macOS and Windows.

   A gate script has a test for each way the gate could pass when it should fail, such as a lost finding or a skipped check, where review would likely miss it.
   Most such faults lie outside the diff: in a bumped tool pin, on a platform the reviewer does not run, or on the staged tree.
   Each case says in a comment which way it guards.
   Other ways are checked by hand when the gate is added or changed, and the pull request records the check.
   A gate that fails when it should pass shows itself the first time it runs, and needs no test.
6. **Workflow safety.**
   Third-party actions are pinned by full commit SHA.
   Workflows default to `permissions: contents: read`, widened per job only where needed.
   Pull requests from forks run on `pull_request`, never `pull_request_target`, and never receive secrets.
   Jobs that need secrets run only on branches in the repository, and a secret reaches only the step that needs it.
7. **Scheduled jobs are not merge gates.**
   Jobs such as probes of harness behaviour run on a schedule, open an issue on failure, and never block a pull request.

## Consequences

- A contributor runs exactly what CI runs with one command, after `mise trust` and `mise install`.
- A fork pull request can merge without the gates that need a secret; their run on `main` catches a problem afterwards.

## Options considered

- **Hooks as the gate:** skipped with `--no-verify`, absent on web edits.
- **A shell script as the entry point, with mise optional:** contributors without mise install each pinned tool by hand, and two ways to run the gates can drift apart.
- **Another task runner (make, just):** it would still need a second tool to install the pins.
- **CI steps written straight into workflow files:** local runs drift from CI.
