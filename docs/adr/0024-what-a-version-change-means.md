# 24. What a version change means

- Status: accepted
- Date: 2026-11-09
- Deciders: Michał Bożek

## Context

CECH versions its catalogue and its CLI apart ([ADR 14](0014-releases-and-versions.md) item 2, [ADR 16](0016-building-the-cech-cli.md) item 2), and a project pins a catalogue release ([ADR 19](0019-per-project-plugin-enablement.md) item 3).
A version helps a project only if it says what an upgrade can break.
The CLI has a clear surface: commands, flags, output and file formats.
A skill has none of its own: it is text an agent follows, so what it promises must be named before its version can mean anything.
[SemVer 2.0.0](https://semver.org/spec/v2.0.0.html) allows any change before 1.0.0.

## Decision

1. **SemVer 2.0.0** for catalogue releases and CLI releases, each against its own surface.
2. **The catalogue's surface** is what a project or another skill relies on:
   - the names of its plugins;
   - the roles it defines and what each means ([ADR 21](0021-roles.md) item 2);
   - the settings each skill reads in `.cech/` ([ADR 17](0017-repository-data-and-state.md) item 2);
   - what each skill's description says it does, on both harnesses ([ADR 10](0010-claude-code-and-codex-from-one-source.md) item 4).
3. **The CLI's surface** is what users, scripts, extensions and plugins rely on:
   - its commands, flags and exit statuses, and the output that [ADR 15](0015-the-cech-cli.md) item 5 makes a contract;
   - the formats of the files it reads and writes, such as `.cech/config.toml` and `data/marketplace.toml`;
   - the layout rules and the categories it checks ([ADR 12](0012-categories-and-names.md) item 1).
4. **Bumps.**
   A major release may remove, rename or narrow anything on a surface, or change what it means.
   A minor release may add to a surface, such as a plugin, an optional setting or a JSON field, and leaves what is there as it was.
   A patch release changes nothing on a surface: a skill's wording may change, but not what it promises, and the CLI's file formats stay as they were.
5. **Before 1.0.0,** the minor number takes the major's place and the patch number the minor's, as Cargo reads SemVer.
6. **A commit marked as breaking** ([ADR 6](0006-change-record.md) item 3) makes the next release a major one, or a minor one before 1.0.0.

## Consequences

- A project can take a patch or minor release without checking every skill it enables.
- A rewording that weakens what a skill promises needs a major release, so review reads a changed description against the old one.
- A config written by a newer minor release of the CLI may hold a key an older one rejects ([ADR 17](0017-repository-data-and-state.md) item 6); a patch release never adds one.

## Options considered

- **Calendar versions for the catalogue:** the number would say nothing about what an upgrade can break.
- **No surface for skills, so every skill change is a minor release:** a project could not tell a fix from a change it must check.
- **Bump rules in each ADR that defines a surface:** they drift apart, and a reader must find them all.
- **SemVer's own reading before 1.0.0:** anything may change, so a pinned project could take no update safely.
