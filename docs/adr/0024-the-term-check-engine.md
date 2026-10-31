# 24. The term check engine

- Status: accepted
- Date: 2026-10-30
- Deciders: Michał Bożek

## Context

The term check of [ADR 3](0003-vendor-neutrality.md) runs in two places.
CECH's gate runs a scanner pinned in mise ([ADR 4](0004-gates-and-ci.md)).
`cech check` runs it in an extension ([ADR 20](0020-the-extension-model.md) item 5), without mise ([ADR 15](0015-the-cech-cli.md) item 7).
[ADR 19](0019-per-project-plugin-enablement.md) keeps credentials out of the files `cech` owns, and its check fails when those files break it (items 1 and 8).

Both read rule files: CECH's in `checks/terms/`, and those an extension lists in `data/marketplace.toml` ([ADR 17](0017-repository-data-and-state.md) item 7).
Extensions' rule files make their format a contract of the CLI ([ADR 15](0015-the-cech-cli.md) item 5).
Maintained rule sets track token formats CECH would not keep up with.
The standard library comes first, unless writing a feature costs more than depending on a module ([ADR 16](0016-building-the-cech-cli.md) item 1).

Checked on 2026-10-29 and 2026-10-30 with Go 1.27.3:

- gitleaks 8.30.1 has no documented library API, and its README says it gets security fixes only; its author now works on Betterleaks.
- Betterleaks 1.9.0, as a Go module, reads CECH's gitleaks-format rule files unchanged, allowlists included, CECH's inline-marker allowlist among them, and can add its 463 default rules to them.
  It links 49 modules and grows the stripped binary from 2.6 MB to 17.7 MB; the maintained rules outweigh that.
- Betterleaks 2.0.0-rc.5 replaces allowlists with expression filters and rejects files in the 1.9.0 format.

## Decision

1. **Betterleaks v1 as a Go module.**
   `cech check` matches with the Betterleaks v1 module, a dependency under [ADR 16](0016-building-the-cech-cli.md) item 1.
   It never sends a found credential to its provider to test it.
   It honours only CECH's exceptions ([ADR 3](0003-vendor-neutrality.md) item 5), not Betterleaks' inline comments or ignore files.
2. **An extension's rule files use Betterleaks v1's format, limited to rules and allowlists.**
   A move to a Betterleaks release that changes the format is a breaking change of the CLI, with a migration note for extensions.
3. **One copy of the generic rules.**
   The generic tier lives only in `cli/internal/terms/generic.toml`, built into the binary, and extends Betterleaks' default rules.
   It holds no address, name or rule that is CECH's own.
   CECH's addresses, vendor names and commit-message rules stay in `checks/terms/`, whose `rules.toml` extends `generic.toml`.
4. **CECH's gate runs the same engine.**
   mise pins the Betterleaks CLI at the version in `cli/go.mod`, so the gate and `cech check` match the same rules the same way.
5. **An extension adds rules and allowlists; it cannot replace or disable a rule.**
   Its allowlists may hide generic hits, such as its own addresses, but never private ones.
6. **The private list comes from the environment.**
   `cech check` reads an extension's private list only from the file named by `CECH_PRIVATE_TERMS`.
   Without a list, the private tier is skipped, as ADR 3 item 4 allows where no copy exists; the output says so, and the skip does not fail the command.
   `CECH_REQUIRE_PRIVATE_TERMS=1` makes a missing list a check that could not run.
7. **What is scanned.**
   In a project, the generic rules scan the files `cech` owns or renders: `.cech/` and the native files.
   In an extension, every tier scans every tracked file as it is in the working tree, and every tracked path.
   A revision range, when given, adds the messages, diffs and paths of its commits, and every ref name and tag message.

## Consequences

- About fifty more modules to keep current.
- A Betterleaks upgrade can bring rules that fail unchanged files, so it lands as a change of its own.
- A token elsewhere in a project is left to the project's own secret scanning.
- CI that gives no revision range skips history and refs, and without `CECH_REQUIRE_PRIVATE_TERMS=1` it can skip the private tier.

## Options considered

- **gitleaks as a Go module:** an upstream that adds no new rules.
- **Betterleaks 2:** still a release candidate.
- **A standard-library matcher with gitleaks' rules:** CECH would own leak detection.
- **A scanner the user installs:** a second tool for every user.
- **Two copies of the generic rules with a gate that compares them:** drift is found, not prevented.
- **Replacing or disabling generic rules:** a pass would say less.
- **A blank private list shipped with each extension:** it passes while checking nothing; once filled, it is committed, leaking the terms, or git-ignored and missing from fresh clones.
- **A default private list path:** an extension that forgets to git-ignore it commits its terms.
- **Scanning a project's whole tree:** CECH would scan code it does not own for secrets.
- **Guessing the commit range:** remotes and clone depth vary.
