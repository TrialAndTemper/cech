# 1. Record architecture decisions

- Status: accepted
- Date: 2026-10-16
- Deciders: Michał Bożek

## Context

A few CECH choices are costly to undo: formats, names, pins, CLI promises.
Contributors need the reason behind each rule.
A log full of small decisions hides the ones that matter, and parallel branches clash on numbers or leave gaps.
On `main`, each ADR's status must say whether it is in force, and one index must list them all.
Until the first plugin merges, nothing is built on the ADRs' exact wording, and the ADRs are still settling.

A contract also has detail, such as keys, flags, messages and workflow steps.
Written into an ADR, that detail hides the decision, pulls review towards edge cases, and must change whenever the code does.

## Decision

1. One decision per file, `docs/adr/NNNN-short-title.md`, in a light [MADR](https://adr.github.io/madr/) form: Context, Decision, Consequences, Options considered.
2. An ADR records a **contract**: a choice that is costly to reverse or that many later changes must follow.
   Work that fits an accepted contract (e.g. a new skill) needs no ADR, unless it adds repo-wide machinery (a CLI command, a hook, a file format) or settles a disputed boundary.
   An ADR holds the decision and the reasons for it, the options rejected, the names and paths others rely on, which changes to them stay compatible, and the rules every later change must keep.
   It leaves other detail to a spec (item 8), and names a key or flag only where the decision depends on it.
3. A pull request writes its ADR as `accepted`; merging it is the acceptance.
   `Date` is the day it was written.
   Later statuses: `superseded by ADR N`, `deprecated`.
4. ADR numbers on `main` run from 1 with no gaps.
   A pull request numbers its ADRs right after the highest on `main`, and renumbers if another merges first.
   CI checks the branch merged onto current `main` and rejects duplicate or missing numbers.
5. Until the first plugin merges, an accepted ADR is edited in place, whatever the change; its number and file name stay, so links keep working, and the commit that makes the edit says why.
   From then on, an accepted ADR is not rewritten; typos and links may be fixed.
   - A refinement (a parameter, an edge case, a rename, an added list entry) goes in a dated `> **Amendment (YYYY-MM-DD).**` note, with its evidence, under the paragraph it changes.
     The status gains `Last amended: <date>`.
   - A different option, one an earlier ADR rejected, or a removed or inverted rule needs a new ADR that supersedes the old one, with links both ways.
6. Under 800 words of prose outside code blocks, plain English.
   Past that, move a second decision or a spec's detail out, not the rejected options.
   Cross-references use the form `[ADR N](NNNN-title.md)` and point only to earlier ADRs, except a `superseded by` status, so the log reads in order.
7. `docs/adr/README.md` lists every ADR.
8. **Material for agents lives in `.ai/`; `docs/` is for people.**
   Each change keeps any working material in `.ai/changes/YYYY-MM-DD-short-title/`.
   A change to detail others rely on has a spec there: rules a test can check, reviewed before or with the code.
   Briefs, research and spike results sit beside it, in subfolders as needed.
   Once tests cover every rule, the spec is dated and its folder moves to `.ai/changes/done/`, where only typos and links change; the code and tests are then the authority.
   Notes quoting private or unpublished sources stay out of git.

## Consequences

- Each ADR reads in one sitting.
- A branch may have to renumber its ADR before merging.
- Until the first plugin merges, an ADR shows only the current decision, and how it got there is in git history.
- A folder in `done/` may no longer match the code.

## Options considered

- **Decisions in pull requests only:** reasons get lost in closed threads.
- **An ADR for every notable change:** the contracts get buried.
- **Amendment notes from the first ADR on:** the groundwork would reach the first plugin as a stack of notes on text nothing yet relied on.
- **Reserve a number when a branch starts:** needs a shared registry, and abandoned branches leave gaps.
- **All detail in the ADR:** each new key or message would need an amendment note.
- **Reference documents:** a copy of the code that goes stale.
- **Specs in `docs/`:** mixes material for agents with documents for people.
- **Folders by kind of note:** spreads one change over several places.
- **Finished specs deleted:** the intent behind a changed test is lost.
- **Finished specs kept current:** needs repeated checks against the code.
- **One merged system specification:** merging loses rules, and parallel changes clash.
