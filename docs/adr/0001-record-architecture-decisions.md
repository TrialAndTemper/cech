# 1. Record architecture decisions

- Status: accepted
- Date: 2026-10-16
- Deciders: Michał Bożek

## Context

A few CECH choices are costly to undo, such as formats, names, pins and CLI promises, and contributors need the reason behind each.
A log full of small decisions hides the ones that matter, and parallel branches clash on numbers or leave gaps.
Each ADR's status must say whether it is in force, and one index must list them all.
Until the first plugin merges, nothing is built on the ADRs' exact wording.
After that, most work fits accepted ADRs, so two pull requests rarely add ADRs at once.

Written into an ADR, contributor rules and the detail users rely on, such as keys, flags and messages, hide the decision, pull review towards edge cases, and change whenever the code or the way of working does.
Agents follow every line of an ADR as if it were a decision.

## Decision

1. One decision per file, `docs/adr/NNNN-short-title.md`, in a light [MADR](https://adr.github.io/madr/) form: Context, Decision, Consequences, Options considered.
2. **An ADR records a decision that is costly to reverse:** what was chosen, why, what follows, and the options rejected.
   A line belongs there only if changing it would need a new decision: an ADR names a key, path, command or flag only where the decision depends on it, and holds no task names, CI layouts, messages or per-item lists.
   Work that fits accepted ADRs, such as a new skill, needs none, unless it adds repo-wide machinery, such as a CLI command, hook or file format, or settles a disputed boundary.
3. A pull request writes its ADR as `accepted`; merging it is the acceptance.
   `Date` is the day it was written.
   Later statuses: `superseded by ADR N`, `deprecated`.
4. ADR numbers on `main` run from 1 with no gaps.
   A pull request numbers its ADRs right after the highest on `main`, and renumbers if another merges first.
   A gate checks this item, item 7 and the word limit of item 6.
5. Until the first plugin merges, an accepted ADR is edited in place, whatever the change, and ADRs may be renumbered into the order they depend on; the commit says why, and a renumbering maps old numbers to new.
   From then on, an accepted ADR is not rewritten; typos, links and wording that keeps the meaning may be fixed, and the commit says so.
   - A refinement, such as a parameter, an edge case or a rename, goes in a dated `> **Amendment (YYYY-MM-DD).**` note with its evidence, under the paragraph it changes, and the status gains `Last amended: <date>`.
   - A different option, one an earlier ADR rejected, or a removed or reversed rule needs a new ADR that supersedes the old one, with links both ways.
6. Under 800 words of prose outside code blocks, plain English.
   Past that, split a second decision into its own ADR and move a rule or detail to its home outside the ADRs; keep every rejected option.
   Cross-references use the form `[ADR N](NNNN-title.md)` and point only to earlier ADRs, except a `superseded by` status, so the log reads in order.
7. `docs/adr/README.md` lists every ADR.

## Consequences

- Each ADR reads in one sitting, and a change to a way of working or an interface's detail leaves the ADRs alone.
- A branch may have to renumber its ADR before merging.
- Until the first plugin merges, an ADR shows only the current decision, and how it got there is in git history.

## Options considered

- **Decisions in pull requests only:** reasons get lost in closed threads.
- **An ADR for every notable change:** the decisions get buried.
- **Amendment notes from the first ADR on:** the groundwork would reach the first plugin as a stack of notes on text nothing yet relied on.
- **Reserve a number when a branch starts:** needs a shared registry, and abandoned branches leave gaps.
- **A placeholder until approval, numbered just before merge:** a clash still means renumbering, only later, and reviewers cannot see where the ADR falls in the order.
- **Dates instead of numbers,** such as `20261109-title.md`: long citations, and ADRs written on the same day can still clash.
- **The pull request's number:** no clashes, but the numbers have gaps, and an ADR opened first can merge last, so the numbers no longer give the reading order.
- **A folder for rejected ADRs:** a rejected idea is kept as an option in the ADR that settled its question, next to the reasons it lost, and a "no" that is costly to reverse is an accepted ADR of its own.
- **All detail and contributor rules in the ADRs:** each new key, message or way of working would need an amendment note.
