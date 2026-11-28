# Team on every plugin line of the code owners file

Implemented 2026-11-26; not maintained: the code and its tests are current.

`cech build` adds the marketplace's maintainer team to each plugin line it writes in `.github/CODEOWNERS`, so a plugin's folders keep a valid code owner when every person its manifest names is invalid.
See [issue 86](https://github.com/TrialAndTemper/cech/issues/86).

The rule is in [the code owners file](../../../docs/reference/generated-files.md#the-code-owners-file), edited with the code.
The change also updates ADR 13, the `team` key in `docs/reference/marketplace.md`, and the code owner rules in `CONTRIBUTING.md`.

## Acceptance criteria

### Plugin lines

1. WHEN a plugin names at least one person, THEN its plugin folder line and its eval folder line name its owner, its maintainers and then the team.
   Test: `cli/internal/build/codeowners_test.go` `TestCodeOwners`, plugins `d-none` and `e-dup`; `cli/internal/app/build_test.go` with the `acme` fixture's `out/.github/CODEOWNERS`.
2. WHEN a plugin names no one, THEN it gets no lines, and the catch-all line gives its folders to the team.
   Test: `cli/internal/build/codeowners_test.go` `TestCodeOwners`, plugins `a-org`, `b-slash` and `c-http`.

## Out of scope

- Checking the names against GitHub; ADR 13 says why.
