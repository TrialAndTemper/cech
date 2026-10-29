# 21. Tool pins

- Status: accepted
- Date: 2026-10-29
- Deciders: Michał Bożek

## Context

The gates run tools that mise installs and puts on `PATH` ([ADR 10](0010-gates-and-ci.md) item 1).
Check tools give different results in different versions, so contributors and CI need the same versions.
A release only days old has had little use, and a fault in it reaches every run at once.

Probed with mise 2026.10.3 on 2026-10-19:

- `mise lock` records each tool's download and checksum per platform, and with `locked` set an install fails on an unlocked version or a checksum mismatch;
- `minimum_release_age` applies when mise resolves a version, not to an exact version given by hand.

## Decision

1. **Exact versions.**
   Every tool that builds or checks CECH is pinned to an exact version in the `[tools]` table of `mise.toml`, with each download and its checksum in `mise.lock`.
   The table holds no harness; the harness data file records the harness versions CECH relies on ([ADR 5](0005-native-tool-features-first.md) item 3).
2. **Locked installs.**
   CI installs an exact mise version, then the tools with `locked` set; `mise.toml` sets mise's `min_version`.
3. **Bumps wait a week.**
   Pins are added and bumped by letting mise resolve them under a week's `minimum_release_age`; review checks the release date of a version given by hand.

## Consequences

- A bump is a visible change to `mise.toml` and `mise.lock`, reviewed like any other.
- A tool's new release reaches CECH no sooner than a week after it is published, unless a version is given by hand.

## Options considered

- **Version ranges, with exact versions only in the lockfile:** a lock refresh could change a tool's version with no visible change to its pin.
