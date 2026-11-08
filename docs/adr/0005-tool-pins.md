# 5. Tool pins

- Status: accepted
- Date: 2026-10-29
- Deciders: Michał Bożek

## Context

The gates run tools that mise installs and puts on `PATH` ([ADR 4](0004-gates-and-ci.md) item 1).
Check tools give different results in different versions, so contributors and CI need the same versions.
A release only days old has had little use, and a fault in it reaches every run at once.

Probed with mise 2026.10.3 on 2026-10-19:

- `mise lock` records each tool's download and checksum per platform, and with `locked` set an install fails on an unlocked version or a checksum mismatch;
- `minimum_release_age` applies when mise resolves a version, not to an exact version given by hand.

## Decision

1. **Exact versions.**
   Every tool that builds or checks CECH is pinned to an exact version in `mise.toml`, and the lockfile records each download's checksum, or, for a Go tool built from source, its version, which the Go checksum database checks.
   Claude Code and Codex are not pinned: CECH tests its work against them, but does not build with them.
2. **Locked installs.**
   CI installs an exact mise version, then the tools with `locked` set, and `mise.toml` names the oldest mise it works with.
3. **Bumps wait a week.**
   Pins are added and bumped by letting mise resolve them under a week's `minimum_release_age`; review checks the release date of a version given by hand.

## Consequences

- A bump is a visible change to the pin and the lockfile, reviewed like any other.
- A tool's new release reaches CECH no sooner than a week after it is published.

## Options considered

- **Version ranges, with exact versions only in the lockfile:** a lock refresh could change a tool's version with no visible change to its pin.
