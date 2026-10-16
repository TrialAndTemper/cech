# 1. Record architecture decisions

- Status: accepted
- Date: 2026-10-16
- Deciders: Michał Bożek

## Context

A few CECH choices are costly to undo: formats, names, pins, CLI promises.
Contributors need the reason behind each rule.
A log full of small decisions hides the ones that matter, and parallel branches clash on numbers or leave gaps.
On `main`, each ADR's status must say whether it is in force, and one index must list them all.

## Decision

1. One decision per file, `docs/adr/NNNN-short-title.md`, in a light [MADR](https://adr.github.io/madr/) form: Context, Decision, Consequences, Options considered.
2. An ADR records a **contract**: a choice that is costly to reverse or that many later changes must follow.
   Work that fits an accepted contract (e.g. a new skill) needs no ADR, unless it adds repo-wide machinery (a CLI command, a hook, a file format) or settles a disputed boundary.
3. A pull request writes its ADR as `accepted`; merging it is the acceptance.
   `Date` is the day it was written.
   Later statuses: `superseded by ADR N`, `deprecated`.
4. ADR numbers on `main` run from 1 with no gaps.
   A pull request numbers its ADRs right after the highest on `main`, and renumbers if another merges first.
   CI checks the branch merged onto current `main` and rejects duplicate or missing numbers.
5. An accepted ADR is not rewritten; typos and links may be fixed.
   - A refinement (a parameter, an edge case, a rename, an added list entry) goes in a dated `> **Amendment (YYYY-MM-DD).**` note, with its evidence, under the paragraph it changes.
     The status gains `Last amended: <date>`.
   - A different option, one an earlier ADR rejected, or a removed or inverted rule needs a new ADR that supersedes the old one, with links both ways.
6. Under 1,000 words, plain English.
   Cross-references use the form `[ADR N](NNNN-title.md)` and point only to earlier ADRs, except a `superseded by` status, so the log reads in order.
7. `docs/adr/README.md` lists every ADR.

## Consequences

- Each ADR reads in one sitting.
- A branch may have to renumber its ADR before merging.

## Options considered

- **Decisions in pull requests only:** reasons get lost in closed threads.
- **An ADR for every notable change:** the contracts get buried.
- **Reserve a number when a branch starts:** needs a shared registry, and abandoned branches leave gaps.
