# 23. Forking upstream skills

- Status: accepted
- Date: 2026-10-29
- Deciders: Michał Bożek

## Context

Some CECH skills adapt text from another open-source project, its *upstream*.
[ADR 2](0002-licence-and-contributions.md) item 4 asks for a compatible licence, upstream's notice, and a source note naming the upstream project and commit.
It does not say which licences are compatible, where the commit is written, or how a fork takes in later upstream changes.

Copyright protects wording, not ideas; CECH credits an idea alone in `CREDITS.md` ([ADR 13](0013-attribution.md) item 3).
When an installer copies a skill folder, its `NOTICE` goes too, and agents do not read it as guidance ([ADR 22](0022-two-install-paths.md) item 4).
Upstream text may name organisations or vendors ([ADR 3](0003-vendor-neutrality.md) items 1 and 4).
Apache-2.0 also asks that copies keep the licence and upstream's notices, and that changed files say so.

Existing tools come first ([ADR 9](0009-native-harness-features-first.md) item 1).
The [GitHub CLI manual](https://cli.github.com/manual/gh_skill_update), read on 2026-10-29 for `gh` 2.103.3, says `gh skill update --force` replaces local edits with upstream's files, so it cannot keep an adapted fork.

## Decision

1. **A fork is any skill with text derived from an upstream.**
   Any passage, list, example or script copied, translated, closely paraphrased or reordered from upstream makes it one.
   A skill written from an upstream's idea alone, not its wording or structure, credits the idea in `CREDITS.md`.
   In doubt, it is a fork.
2. **Allowed licences**, by SPDX id: `MIT`, `BSD-2-Clause`, `BSD-3-Clause`, `ISC`, `0BSD`, `Apache-2.0`, `CC0-1.0`.
   Each can be met inside the skill folder, and none is copyleft.
   Text with no licence, or under any other licence, is not taken into a skill; adding a licence to the list changes this ADR.
3. **The skill's `NOTICE` is the source note**, with one *entry* per upstream.
   An entry starts with fixed lines: upstream's repository URL, the *pinned commit* (the full hash of the last upstream commit taken in), the upstream paths the text came from, and the licence's SPDX id.
   Next comes a line saying whether CECH changed the text, then upstream's copyright and licence text word for word, and for `Apache-2.0` also upstream's own `NOTICE` text.
   The notice is never rewritten to pass the term check; a hit goes in its allowlist ([ADR 3](0003-vendor-neutrality.md) item 5).
   `plugin.json` `license` is an SPDX expression naming every licence in the folder, such as `MIT AND Apache-2.0`.
4. **Files from `Apache-2.0` upstreams keep upstream's notices in them, and changed ones say so.**
   Each changed file carries a one-line comment naming its upstream and saying CECH changed it; review checks which files need one.
5. **`cech validate` checks the entries offline**, and that comments match entries marked changed, in the merge gate ([ADR 4](0004-gates-and-ci.md) item 5).
6. **A scheduled job checks upstreams** ([ADR 4](0004-gates-and-ci.md) item 7).
   It reports a pinned commit that upstream no longer has, and listed paths that changed upstream after it.
7. **Updates are diffed and applied by hand.**
   A contributor diffs upstream's paths from the pinned commit to a newer one, and applies only the changes CECH wants, rewritten to its rules.
   That change moves the pinned commit, updates the entry, and names the old and new upstream commits.
   The reviewer reads the change beside that upstream range.
8. **A fork may diverge without limit.**
   CECH's rules win over upstream wording: a fork drops any name ADR 3 bars and any rule CECH would not keep.
   Its entry stays while any derived text remains.
   A change may remove an entry only if it says no derived text is left; it then moves upstream's credit to `CREDITS.md`.

## Consequences

- A fork's origin and licence texts sit in one file that travels with the skill.
- Updates are by hand, prompted by the scheduled report.
- Upstreams under other licences, such as copyleft or CC BY, can inspire a skill but not be forked.

## Options considered

- **CC BY 4.0 on the list:** the [Creative Commons FAQ](https://creativecommons.org/faq/#can-i-apply-a-creative-commons-license-to-software), read on 2026-10-29, advises against CC licences for software, such as a skill's scripts.
- **Copyleft upstreams:** their terms can bind organisations that extend CECH.
- **A fork only for a substantial part, or as the contributor declares:** the same borrowing is judged differently from skill to skill.
- **A separate data file:** a third hand-written file per plugin ([ADR 18](0018-generated-files.md) item 1), beside `NOTICE`.
- **`plugin.json` or `SKILL.md` `metadata`:** an installer drops the first; agents load the second.
- **Free prose in `NOTICE`:** no gate can read it.
- **Dropping `Apache-2.0`, or listing changed files only in `NOTICE`:** the first loses Apache-2.0 upstreams; the second may miss Apache-2.0's notice in each changed file.
- **An unchanged upstream copy to merge from:** it does not serve the skill, so a plugin may not hold it ([ADR 11](0011-one-plugin-one-skill.md) item 2).
