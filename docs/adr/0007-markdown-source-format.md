# 7. Markdown source format

- Status: accepted
- Date: 2026-10-19
- Deciders: Michał Bożek

## Context

People review changes as raw source in diffs, often on a phone.
The layout of the source decides how readable those diffs are and how much of a paragraph a small edit marks as changed.

Checked on 2026-10-19 (rumdl 0.3.0, MIT licence): rumdl, a Markdown linter based on markdownlint's rules, checks and rewrites Markdown to one sentence per line.
Applied to the twelve ADRs then written, it changed no rendered output.
It leaves YAML frontmatter alone and does not split after "e.g.".
Block disable comments and an exclude list in its configuration exempt parts of files or whole files.
It is still at 0.x, so a new release may format files differently.

## Decision

1. **Scope.**
   The rule covers the body of every tracked Markdown file: docs, ADRs, the README and contributing guides, `SKILL.md` files and their references, and eval prompts.
   YAML frontmatter is outside it.
   Generated Markdown follows it too.
2. **One sentence per line.**
   A sentence is never broken across lines.
   A sentence that is too long is shortened by rewriting it, not by wrapping it.
3. **rumdl checks and fixes it.**
   `.rumdl.toml` at the repository root turns on sentence-per-line reflow and removes the line-length limit; every other setting is rumdl's default.
   `rumdl check` is the Markdown lint gate of [ADR 4](0004-gates-and-ci.md), and `rumdl fmt` fixes files.
   Its version is pinned with the other check tools.
4. **Upgrades are their own commit.**
   An upgrade and the reformat it causes land together in one commit.
5. **Exceptions.**
   Text copied verbatim from upstream keeps its own layout: a whole file goes in the exclude list of `.rumdl.toml`, a part of a file sits between block disable comments, and each exception gives its reason next to it.
6. **Commit messages are separate.**
   Their bodies stay wrapped at 72 characters, as [ADR 6](0006-change-record.md) says; this rule does not change that.

## Consequences

- Diffs show whole changed sentences, and nothing else.
- Line lengths vary; sentences with several links run long, and soft-wrap on a narrow screen.
- One more check tool to pin and upgrade.
- If rumdl goes a year without a release, or splits sentences wrongly in a way its configuration cannot fix, a new ADR chooses another tool for the same rule.

## Options considered

- **Wrap at a fixed width:** folds into odd fragments on narrow screens, and a one-word edit reflows the whole paragraph.
- **One line per paragraph:** hard to read in an editor, and every diff is as large as the paragraph.
- **Sentences plus breaks at clauses:** brings back the short fragments, and where to break is a matter of taste.
- **A CECH reflow script:** it worked on the ADRs, but it is code CECH would have to maintain.
- **Prettier with a sentence-per-line markdownlint rule:** two tools and a Node toolchain for the same result.
