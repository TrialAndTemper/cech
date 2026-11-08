# 24. Where each kind of text lives

- Status: accepted
- Date: 2026-11-08
- Deciders: Michał Bożek

## Context

[ADR 1](0001-record-architecture-decisions.md) item 2 keeps contributor rules and interface detail out of the ADRs, so they need homes of their own.
Users, scripts and extensions rely on the detail of CECH's files, commands and checks, and need a page that says what is true now.
A copy of what the code does can fall behind it, and agents follow whatever a document says, so a page that describes planned behaviour as if it existed misleads them.
A change also needs a place for its rules before the code exists, for its test plan, and for the research behind it, whether a person or an agent writes them.

## Decision

1. **How contributors work** is in `CONTRIBUTING.md`.
2. **What CECH does now** is in `docs/reference/`, in pages on the files, commands and checks that others rely on.
   A page describes only what exists, and changes in the same commit as the behaviour it describes.
   Where a page and the code disagree, the code and its tests are right, and the page is fixed.
3. **A change's working material** is in `changes/YYYY-MM-DD-short-title/`, written by people and agents alike.
   - A change to behaviour that a reference page describes edits that page, and the edit is its spec.
   - Any other rule a change adds, such as how a gate behaves or what a command not yet built must do, gets a spec in the folder, reviewed before or with the code.
   - The folder records the test that covers each rule, and may hold briefs, research and spike results.
   - An ADR states each finding it rests on in one sentence, with its date and the versions checked; the steps and raw results stay in the research.
   - Once a test, or a recorded check by hand, covers every rule, the folder moves to `changes/done/`, where only typos and links change.
4. An ADR or a reference page never links into `changes/`, since its folders move; it states what it rests on in its own words.
5. Notes quoting private or unpublished sources stay out of git ([ADR 3](0003-vendor-neutrality.md)).

## Consequences

- A change to how contributors work, or to an interface's detail, leaves the ADRs alone.
- A reference page can fall behind the code; review asks for it with each change.
- Behaviour that is not built yet is found in `changes/`, not in `docs/`.
- A folder in `done/` may no longer match the code.

## Options considered

- **No reference, only code and finished specs:** users and extensions have nothing current to read, and the detail creeps back into the ADRs.
- **A reference generated from the code only:** rules written in prose, such as which changes stay compatible, have no home.
- **Planned behaviour in the reference, marked as such:** readers and agents take it for what exists.
- **A folder per change and a separate spec for each interface change:** each rule is written twice, and the copies drift.
- **Change folders in `docs/`:** finished specs that no longer match the code would sit beside the current documents.
- **A hidden folder for agents' material only:** people doing the same work by hand would not see it as theirs.
- **Folders by kind of note:** spreads one change over several places.
- **Finished specs deleted:** the intent behind a changed test is lost.
- **Finished specs kept current:** needs repeated checks against the code; the reference holds what must stay current.
- **One merged system specification:** merging loses rules, and parallel changes clash.
