# 4. Vendor neutrality

- Status: accepted
- Date: 2026-10-16
- Deciders: Michał Bożek

## Context

Organisations extend CECH with marketplaces of their own.
If the core holds one private organisation's names, hosts or products, it is no longer neutral and may leak private details.
A leak in public git history is hard to undo.

## Decision

1. **No content specific to a private organisation** in tracked files or in git metadata (commit messages, branch and tag names, release notes).
   This means its internal names, hosts, emails, products, ids, private tracker keys, and quotes from private sources.
   - Allowed: author, `Signed-off-by:` and co-author identities, credits allowed by [ADR 3](0003-licence-and-contributions.md), and CECH's own `#N` issue references.
2. **Examples use invented data:** `example.com`, `*.example`, `*.test`, the company `acme`.
   A skill about a public product may name it.
3. **Organisation needs become generic ideas.**
   A private tracker becomes the `issue-tracker` role with a public adapter.
   A brand becomes a brand contract with an invented demo.
   The organisation's own version lives in its extension.
4. **A term check enforces this,** before each commit, on each commit message, and in CI.
   Its public tiers live in git: generic rules for shapes that suggest copied private content, and a list of vendor names CECH does not cover.
   Its private tier lives outside git, in each contributor's own list and in a CI secret, and a hit never reveals the term.
   Pull requests from forks never receive the secret and run only the public tiers.
   A false positive on a public tier is allowed in git with its reason; an exception to a private term stays in the private list.

## Consequences

- Fork pull requests are not checked against private terms.

## Options considered

- **Review only:** people miss things, above all in commit messages.
- **Hashed private terms in git:** short terms are easy to guess; extra machinery for little secrecy.
- **All terms private:** fork pull requests would never be checked.
