# 12. Attribution

- Status: accepted
- Date: 2026-10-17
- Deciders: Michał Bożek

## Context

Each plugin needs a named person who answers for it, and GitHub should ask that person to review changes to it.
Credit for ideas follows [ADR 2](0002-licence-and-contributions.md).
If the same names are kept in several hand-written files, they drift.
CECH starts with one maintainer.

Checked on 2026-10-17 (Claude Code 2.1.302):

- `plugin.json` `author` is a single object with `name`, `email` and `url`.
  `claude plugin validate --strict` rejects an array or a string.
- An unknown field such as `maintainer` or `maintainers` is a warning, so strict validation fails.
- A `metadata` map in `SKILL.md`, an [Agent Skills](https://agentskills.io/specification) field, passes.

[GitHub docs](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners), read on 2026-10-17: code owners must have write access to the repository, the last matching pattern wins, and GitHub reports invalid entries.

## Decision

1. **Owner.**
   `plugin.json` `author` names the plugin's owner: the person who answers for it now, not necessarily who wrote it first.
   They choose the name and whether to give an `email` ([ADR 2](0002-licence-and-contributions.md)).
   A plugin with no individual owner names the CECH maintainers, with the organisation's GitHub page as `url`.
2. **Further maintainers** go in `SKILL.md` frontmatter as `metadata.maintainers`: GitHub user names separated by spaces.
   There is no `maintainer` field in `plugin.json`.
   This works around the validator and is recorded in the harness data file ([ADR 5](0005-native-tool-features-first.md)); it ends if `claude plugin validate --strict` accepts a maintainers field.
3. **Credit for ideas** and earlier authors, as [ADR 2](0002-licence-and-contributions.md) allows, goes in `CREDITS.md` in the skill folder, next to any source notes for third-party text.
   It counts as an asset of the skill under [ADR 7](0007-one-plugin-one-skill.md).
   Credits are for people, not agents, so `SKILL.md` does not link to the file.
4. **Generated code owners.**
   `cech build` ([ADR 9](0009-generated-files.md)) writes `.github/CODEOWNERS`:
   - first, a catch-all line for the marketplace's maintainer team, the `team` in its `data/marketplace.toml` (for CECH, `@TrialAndTemper/maintainers`);
   - then, for each plugin, its folder (and its evals folder, once a later ADR on evals places it), owned by the owner and the maintainers.

   A marketplace with no `team` gets no code owners file.
   The build reads the owner's user name from a `url` of the form `https://github.com/<user>`, without network access.
   It leaves the owner out when `<user>` is the organisation of the catch-all team, or when the `url` has another form or is missing.
   A plugin whose line would then name nobody gets no line, and the catch-all team owns it.
   Any other organisation given as `url` is caught by the gate in item 5.
5. Being named an owner or maintainer is a request for write access.
   A CECH maintainer grants it, or the name is removed before merge.
   A gate ([ADR 10](0010-gates-and-ci.md)) fails when GitHub reports code owners errors for the change.
   It reads them with a token, so it follows ADR 10's rule for gates that need a secret.
6. Code owner review is requested, not required, until CECH has two maintainers on the team; with one, a required review would block every change they make.
7. **Git history is separate.**
   Commit authors and `Co-authored-by:` trailers ([ADR 11](0011-change-record.md)) record who changed what.
   The fields above record who answers for a plugin now.

## Consequences

- Changing a plugin's owner is a change to its manifest, reviewed like any other.
- Owners and maintainers need GitHub accounts with write access to get review requests.
- A one-person plugin has no backup reviewer until the team adds one.

## Options considered

- **A `maintainers` field in `plugin.json`:** fails strict validation (checked above).
- **A hand-written maintainers file or code owners file:** drifts from the manifests.
- **Owners derived from git history:** commits show who changed a plugin, not who answers for it now, and some come from bots or tools.
