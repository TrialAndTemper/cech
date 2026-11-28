# 26. Eval runs in containers

- Status: accepted
- Date: 2026-11-26
- Deciders: Michał Bożek

## Context

[ADR 25](0025-evals.md) item 6 has `cech eval` drive both harnesses, with each arm out of the other's reach.
A release needs hundreds of model runs, so where they run and on whose account decides what they cost.

Measured on 2026-11-26 with Claude Code 2.1.332 and Codex CLI 0.171.0, on a test plugin:

- a run took about 10 s on Claude Code and 13 to 20 s on Codex, and one login ran 6 Claude Code or 4 Codex runs at once without error;
- in 6 of 10 Codex runs without the plugin, Codex searched the disk, found the plugin's skill on the host and followed its instructions.

A run holds a live credential, and a skill under test can tell the model to run any command.

Read on 2026-11-26, with those versions installed:

- `claude setup-token` makes a one-year token for scripts and CI that draws on the user's subscription.
- Codex's docs allow a ChatGPT login in CI only on trusted private runners, never for public or open-source repositories; an API key is billed per token.
- Codex refreshes a login older than about 8 days and writes it back, and its docs warn against one login file shared by concurrent jobs.

CECH cannot pay for API keys.

## Decision

1. **One container per run.**
   `cech eval` starts every run in a fresh container from one image, pinned by digest.
   The image pins each harness's version, as [ADR 5](0005-gates-and-tool-pins.md) item 8 allows.
   The container sees only that run's workspace and home folder, and reaches only the hosts its harness needs.
   A harness whose licence forbids passing it on, such as Claude Code, stays out of the image: `cech eval` installs the version the image pins on the maintainer's machine and lends it to each run.
2. **Arms run alike.**
   A case's arms run in one batch: on one commit, with the same image, harness, model and `cech` version, whether one `cech eval` command makes the runs or several do.
3. **One credential per run.**
   `cech eval` hands each run one credential and writes none into the results.
   It is an API key or a maintainer's own subscription login: a `claude setup-token` token on Claude Code, and on Codex a copy of a ChatGPT login refreshed before the batch.
4. **The same command everywhere.**
   The same `cech eval` command and image can run on a maintainer's machine or in CI.
   Until CECH has API keys, model runs happen on a maintainer's machine, on that maintainer's logins.

## Consequences

- A machine that runs evals needs Docker or Podman, and CECH builds and pins the image.
- Isolation does not depend on a harness's own sandbox.
- Runs on a subscription count against that maintainer's plan limits.
- Moving runs to CI needs API keys, at least for Codex.

## Options considered

- **Runs on the host, in separate folders:** Codex reads outside its workspace, as the runs above showed.
- **Sandcat** (VirtusLab, at commit `405ed10`, read on 2026-11-26): built for one interactive dev-container session, with no release to pin, Docker only, tool versions left unpinned and cache volumes shared across runs.
- **Anthropic's sandbox-runtime** (0.0.83, read on 2026-11-26): limits a process's files and network without a container, but runs the host's harness versions, and supports Windows only in alpha.
- **API keys in CI now:** no money for them.
- **Claude Code in the published image:** its licence reserves all rights, so CECH may not pass it on.
- **A maintainer's subscription logins in CI:** Codex's docs rule this out for a public repository, and a Claude Code token is tied to one person's plan.
