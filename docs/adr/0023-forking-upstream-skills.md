# 23. Forking upstream skills

- Status: accepted
- Date: 2026-10-29
- Deciders: Michał Bożek

## Context

Some CECH skills adapt text from another open-source project, its *upstream*.
[ADR 3](0003-licence-and-contributions.md) item 4 sets which licences CECH accepts, and asks for upstream's notice and a source note naming the upstream project and commit.
It does not say where the commit is written, or how a fork takes in later upstream changes.

Copyright protects wording, not ideas; CECH credits an idea alone in `CREDITS.md` ([ADR 13](0013-attribution.md) item 3).
When an installer copies a skill folder, its `NOTICE` goes too, and agents do not read it as guidance ([ADR 22](0022-two-install-paths.md) item 4).
Upstream text may name organisations or vendors ([ADR 4](0004-vendor-neutrality.md) items 1 and 4).
Apache-2.0 also asks that copies keep the licence and upstream's notices, and that changed files say so.

Existing tools come first ([ADR 9](0009-native-harness-features-first.md) item 1).
The [GitHub CLI manual](https://cli.github.com/manual/gh_skill_update), read on 2026-10-29 for `gh` 2.103.3, says `gh skill update --force` replaces local edits with upstream's files, so it cannot keep an adapted fork.

## Decision

1. **A fork is any skill with text derived from an upstream.**
   Any passage, list, example or script copied, translated, closely paraphrased or reordered from upstream makes it one.
   A skill written from an upstream's idea alone, not its wording or structure, credits the idea in `CREDITS.md`.
   In doubt, it is a fork.
   A skill in an extension that starts from a CECH skill is a fork, with CECH as its upstream.
2. **Only licences that [ADR 3](0003-licence-and-contributions.md) item 4 accepts.**
   Text with no licence, or under any other licence, is not taken into a skill.
3. **The skill's `NOTICE` is the source note**, with one *entry* per upstream.
   An entry records, in lines a gate can read, where the text came from (the upstream and its paths), the *pinned commit* (the last upstream commit taken in), its licence and whether CECH changed it, then upstream's notices word for word.
   `plugin.json` `license` names every licence in the folder.
4. **Files from `Apache-2.0` upstreams keep upstream's notices in them, and changed ones say so.**
   Each changed file carries a one-line comment naming its upstream and saying CECH changed it; review checks which files need one.
5. **`cech validate` checks the entries offline**, and that comments match entries marked changed, as a gate ([ADR 5](0005-gates-and-tool-pins.md) item 4).
6. **A scheduled job checks upstreams** ([ADR 5](0005-gates-and-tool-pins.md) item 6).
   It reports a pinned commit that upstream no longer has, and listed paths that changed upstream after it.
7. **Updates are diffed and applied by hand.**
   A contributor applies only the upstream changes CECH wants, rewritten to its rules, and moves the pinned commit; a reviewer reads the change beside that upstream range.
8. **A fork may diverge without limit.**
   CECH's rules win over upstream wording: a fork drops any name [ADR 4](0004-vendor-neutrality.md) bars and any rule CECH would not keep.
   Its entry stays while any derived text remains.

## Consequences

- A fork's origin and licence texts sit in one file that travels with the skill.
- Updates are by hand, prompted by the scheduled report.
- Upstreams under licences ADR 3 does not accept, such as copyleft or CC BY, can inspire a skill but not be forked.

## Options considered

- **A fork only for a substantial part, or as the contributor declares:** the same borrowing is judged differently from skill to skill.
- **A separate data file:** a third hand-written file per plugin ([ADR 18](0018-generated-files.md) item 1), beside `NOTICE`.
- **`plugin.json` or `SKILL.md` `metadata`:** an installer drops the first; agents load the second.
- **Free prose in `NOTICE`:** no gate can read it.
- **Dropping `Apache-2.0`, or listing changed files only in `NOTICE`:** the first loses Apache-2.0 upstreams; the second may miss Apache-2.0's notice in each changed file.
- **An unchanged upstream copy to merge from:** it does not serve the skill, so a plugin may not hold it ([ADR 11](0011-one-plugin-one-skill.md) item 2).
