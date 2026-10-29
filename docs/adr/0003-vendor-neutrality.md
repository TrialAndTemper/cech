# 3. Vendor neutrality

- Status: accepted
- Date: 2026-10-16
- Deciders: Michał Bożek

## Context

Organisations extend CECH ([ADR 8](0008-a-marketplace-you-extend-not-fork.md)).
If the core holds one private organisation's names, hosts or products, it is no longer neutral and may leak private details.
A leak in public git history is hard to undo.

## Decision

1. **No content specific to a private organisation** in tracked files or in git metadata (commit messages, branch and tag names, release notes).
   This means its internal names, hosts, emails, products, ids, private tracker keys, and quotes from private sources.
   - Allowed: author, `Signed-off-by:` and co-author identities, credits allowed by [ADR 2](0002-licence-and-contributions.md), and CECH's own `#N` issue references.
2. **Examples use invented data:** `example.com`, `*.example`, `*.test`, the company `acme`.
   A skill about a public product may name it.
3. **Organisation needs become generic ideas.**
   A private tracker becomes the `issue-tracker` role with a public adapter.
   A brand becomes a brand contract with an invented demo.
   The organisation's own version lives in its extension.
4. **A term check enforces this** in pre-commit, on commit messages and in CI, in three tiers:
   - **generic rules** (public): email addresses outside an allowlist, cloud resource ids, signed URLs, single-sign-on user formats, and tracker keys in commit messages.
     The allowlist holds example domains and the author, sign-off, co-author and credited addresses.
     It also holds the contact addresses that CECH's own files publish, such as the email fallback for security reports;
   - **vendor names** (public): a plain list of chosen product and brand names CECH does not cover, since such a name hints at whose private stack the content came from.
     Only listed names are flagged;
   - **private terms**: kept outside git and in a CI secret.
     A hit reports file, line and term number, never the term.
     This tier runs in pre-commit where a local copy exists, on pull requests from branches in the repository, and on `main`.
     It must pass before each release tag, and scans the full history once before the repository goes public.
5. A false positive on a public tier (a common word that is a listed name, a deliberate mention, an address in an upstream notice) goes in a tracked allowlist or carries an inline marker with a reason.
   Exceptions to private terms stay in the private list.
6. `cech check` runs the same tiers with a project's or extension's own lists.

## Consequences

- Fork pull requests are not checked against private terms.

## Options considered

- **Review only:** people miss things, above all in commit messages.
- **Hashed private terms in git:** short terms are easy to guess; extra machinery for little secrecy.
- **All terms private:** fork pull requests would never be checked.
