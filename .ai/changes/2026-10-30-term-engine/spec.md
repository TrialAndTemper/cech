# Term tiers in `cech check` and in CECH's gate

The rule files, the engine settings, the private list and the scan scope that [ADR 24](../../../docs/adr/0024-the-term-check-engine.md) decides, for the term tiers of `cech check` and for CECH's own term gate.
The package `cli/internal/terms` and CECH's gate follow these rules.
The change that adds the `cech check` command calls the package, and builds what the package leaves to it: reading the environment for items 19 and 22; the output of items 22 and 24 on stderr and in `--json`; item 25; the exit statuses of items 24, 27, 29 and 32, exit 2 for `--range` outside an extension included; item 30's `--json` field names; and the tests of items 36 to 44.
[research.md](research.md) holds the measurements behind the decision.

The items are numbered; "item N" below means one of them, never a term rule.

## Files

1. `cli/internal/terms/generic.toml` holds the generic tier and is built into the binary.
   It starts from the `checks/terms/rules.toml` that came before it and keeps `[extend] useDefault = true`, the inline-marker allowlist, the `email` rule with its allowlist of example domains, forge no-reply addresses and public forge remotes, and the `aws-arn`, `azure-subscription`, `gcp-project`, `signed-url` and `sso-user` rules.
   It leaves out every address of CECH's own authors, co-authors, credited people and contacts, and the allowlist for the paths of `checks/terms/`.
2. `checks/terms/rules.toml` holds only what is CECH's own: `[extend] path = "cli/internal/terms/generic.toml"`, a path from the repository root; a global allowlist of CECH's addresses; and the allowlist for the rule files in `checks/terms/`.
   A test fails when an address in it is also in `generic.toml`.
   The rule-file allowlist also covers `cli/internal/terms/generic.toml`, whose regex text, such as the guest-user marker that `sso-user` looks for, matches its own rules; so neither the vendor names nor the default rules scan that file.
3. `checks/terms/messages.toml` and `checks/terms/names.txt` keep their rules; `cech check` never reads them.
4. The comment at the top of each rule file names Betterleaks, not gitleaks.

## Engine

5. `cli/go.mod` requires `github.com/betterleaks/betterleaks` at a v1 version, 1.9.0 or later, and [ADR 16](../../../docs/adr/0016-building-the-cech-cli.md) item 1 lists it in the same change, with its reason.
6. `mise.toml` pins `betterleaks` at the same version and drops `gitleaks`; `mise lock` records it.
   A bump changes both in one commit, under ADR 5's rules for pins.
   `mise run terms` exits 2, naming both versions, when the pinned CLI's version differs from the one in `cli/go.mod`.
7. `cech` never enables Betterleaks' validation or analysis, and makes no network request for the term tiers.
8. `cech` sets the engine to ignore `betterleaks:allow` and `gitleaks:allow` comments, and never reads a `.betterleaksignore` or `.gitleaksignore` file.
   CECH's gate refuses to run while either ignore file exists, as it did for `.gitleaksignore` alone.
9. `cech` checks every extension rule file against items 11 to 16 before it builds the engine, so an invalid file is reported as a finding.
   Should the engine still refuse the rules, that too is a rule-file finding, not a crash.
   The configs are parsed without a path, since the engine skips any scanned file at its config's path.
10. Every regular expression is compiled with Go's `regexp` package, which the engine uses.

## An extension's term lists

11. `terms.rules` and `terms.names` in `data/marketplace.toml` are arrays of paths from the repository root, as the `cech build` spec sets.
    Each path must name a tracked regular file inside the repository, not a symlink, and appear once; otherwise the entry is a finding that names it.
    An empty or absent array adds nothing.
12. A file under `terms.rules` may hold only these keys:
    - at the top level: `title`, `[[rules]]` and `[[allowlists]]`;
    - in a rule: `id`, `description`, `regex`, `keywords`, `path`, `entropy`, `secretGroup`, `tags` and `[[rules.allowlists]]`;
    - in an allowlist: `description`, `condition`, `regexTarget`, `regexes`, `paths` and `stopwords`.

    Any other key, such as `extend`, `filter`, `prefilter`, `validate`, `components` or `required`, is a finding that names the file and the key.
13. A rule's `id` starts with the extension's namespace and `-`, and is unique across every file under `terms.rules`.
    An id that is also a generic or default rule's id, or that starts with `name-` or `private-term-`, is a finding.
14. A rule needs `id`, and `regex` or `path`; a `secretGroup` is from 0 to the number of groups in its `regex`.
    An allowlist needs at least one of `regexes`, `paths` and `stopwords`.
15. Every `regex`, allowlist regex and allowlist path must compile; a finding names the file and the rule id.
16. A file under `terms.names` has one name per line.
    Carriage returns and white space around a name are dropped, and blank lines and lines starting with `#` are ignored.
    Each name becomes the rule `name-<k>-<n>`, where `<k>` is the file's place in `terms.names`, counting from 1, and `<n>` is the line number.
    A name of `-`, `_` and white space alone is a finding, since it would match everywhere.
    It matches as CECH's gate matches `names.txt`: without regard to case; with spaces, `-` and `_` between words matching each other or nothing; and, where a name starts or ends with a word character, with a word boundary there.
17. The public tiers skip the files listed under `terms.rules` and `terms.names`; the private tier scans them.
18. An extension allowlist may hide hits of the generic, default and extension rules.
    A broad one, such as a global `paths = ['.*']`, hides them all; that is the extension's own reviewed choice, and `cech` does not judge it.
    A global path allowlist, like the path filter in Betterleaks' default rules (lock files, images, vendored code), drops a file before any rule runs, as it does in CECH's gate.

## The private list

19. In an extension, `cech check` reads the private list from the file named by `CECH_PRIVATE_TERMS`; there is no default path.
    In a repository that is not an extension, it reads neither `CECH_PRIVATE_TERMS` nor `CECH_REQUIRE_PRIVATE_TERMS`.
    CECH's gate keeps its own default, `.private/denylist.txt`.
20. The format is that of CECH's gate: one regular expression per line, matched without regard to case, with carriage returns and surrounding white space dropped, and blank lines and lines starting with `#` ignored.
    Line `n` becomes the rule `private-term-<n>`.
21. The private tier runs only these rules: no default or generic rule, and no allowlist, inline marker or extension allowlist.
22. When `CECH_PRIVATE_TERMS` is unset or empty, names a missing file, or names a list with no terms, the private tier is skipped, and the output says so on stderr and in `--json`; the skip alone leaves the exit status 0.
    With `CECH_REQUIRE_PRIVATE_TERMS=1`, any of these makes the private tier a check that could not run.
23. An unreadable list, a line that does not compile, a line holding `'''`, or a list the engine refuses also makes it a check that could not run; the message names the line number, never its text, and never quotes the engine's own message.
24. When the private tier could not run, the public tiers still run, since a failure there is worth reporting.
    But no path, file name, key, ref name or range then appears anywhere in the output, `--json` included: each finding shows only its rule id or kind, since any of these could hold a private term.
    `cech check` then exits 1 if anything else failed, else 3, and names both, as ADR 15 item 5 sets.

## What is scanned

25. A repository is an extension when its `data/marketplace.toml` has a `cech` key, and a project when it has `.cech/config.toml`; it can be both.
    A `data/marketplace.toml` without a `cech` key is CECH's own (ADR 17 item 7), and there `cech check` runs no term tier; CECH's gate does that.
26. In a project, the generic rules, without any extension's lists, scan every file under `.cech/` and the native files `.claude/settings.json` and `.codex/config.toml`, as they are in the working tree, tracked or not.
    A symlink there is scanned as its target path, never followed.
27. In an extension, each tier scans the content in the working tree of every file in `git ls-files`, and every tracked path as text.
    The tiers scan paths before contents.
    A symlink is scanned as its target path, never followed; a submodule is skipped.
    A file with a NUL byte in its first 8,000 bytes is skipped as binary; the output counts skipped files.
    A tracked file missing from the working tree is a partial success: `cech check` names it and exits 1.
    Outside a git work tree, an extension's check fails with exit 1.
    The working tree is scanned, not the index as in CECH's gate, since `cech check` checks what CI checks out.
28. In a repository that is both, a hit in `.cech/` that items 26 and 27 both find is reported once.
29. `--range <revision range>` adds, for each commit in the range: its message, the lines its diff adds against each parent, and the paths it touches.
    It also adds the names of local branches, tags and remote-tracking refs, and the messages of annotated tags.
    A value that starts with `-` exits 2, and the message does not repeat it; a range git cannot resolve, such as one cut off by a shallow clone, exits 3 and names the range, unless the private rules match the range text or item 24 or 31 hides details, in which case it says only that the range could not be resolved; an empty range adds no commits and says so.
    `--range` in a repository that is not an extension exits 2.

## Output and exit status

30. Each term finding names its tier (`generic`, `extension` or `private`), the rule id, where it is, and for a public tier the matched text, quoted as Go's `%q` quotes it.
    Where it is: a path and line; the commit for a hit from `--range`; `paths` for a hit in the list of paths; `commit message`, `ref names` or `tag messages` for those.
    The `--json` field names are set with the rest of `cech check`'s output.
31. A private hit shows `private term <n>` and never the term or the matched text.
    Once any private hit is in a path or a ref name, every other private hit shows only the term number and the commit, and no path, ref name or range appears anywhere in the output, `--json` included.
    A private hit in the list of paths or in the ref names keeps that place and its line, since the line shows no text.
    While any private hit exists, public hits show only their rule id, problems show only their kind, and an unresolved range is not named, since an entry, key, rule id or range could hold a term.
    The same holds for a project's findings merged into an extension's result (item 28).
    No message from an engine run with the private list reaches the output.
32. A term hit, an invalid entry under `terms` or an invalid rule file exits 1.
    Any problem under items 11 to 16 stops the generic and extension tiers of item 27, which load together, and the output says they were skipped; the private tier and the project scan of item 26 still run.
    Should the scan of the generic and extension tiers fail, those tiers could not run, and the private results are kept; a failed project scan likewise could not run.
    Exit statuses otherwise follow [ADR 15](../../../docs/adr/0015-the-cech-cli.md) item 5.

## CECH's gate

33. `scripts/check-terms` runs `betterleaks` with the options it gave gitleaks, plus `--regex-engine stdlib`, and needs `betterleaks` on `PATH`.
    The Betterleaks CLI matches with `go-re2` by default and the module with Go's `regexp`, so without that option the two could match differently.
    The gate keeps its own exit statuses (ADR 4): 0 clean, 1 hits, 2 could not run, an unresolvable range included, where `cech check` exits 3.
34. Each config it writes copies `checks/terms/rules.toml` with the `[extend] path` made absolute, since Betterleaks resolves it against the working directory and the script runs each scan from a scratch directory.
    On Windows it writes the path in the form native programs read, as `cygpath -m` gives it under Git Bash; CI's Windows leg runs the gate, which checks it, since bats does not run there (ADR 4 item 5).
35. `test/terms.bats` keeps its guards, now named for Betterleaks, and adds a case for item 6.

## Tests for `cech check`

36. A fixture extension passes with a forge no-reply address and a `.test` address in its files and commit messages, and fails on CECH's author address, on a name from its `terms.names`, and on a term from its private list, reported by number only.
37. A fixture extension allowlist that matches a private term does not hide the private hit.
38. A fixture project fails when `.cech/config.toml` or a native file holds a token that Betterleaks' default rules find, and passes when the same token is in another tracked file.
39. A rule file with a key outside item 12, an id outside item 13 or a regex that does not compile exits 1 with the file named, and `--json` still prints one object.
40. With `CECH_REQUIRE_PRIVATE_TERMS=1` and no list, a clean extension exits 3, and one with a public hit exits 1 and names both.
41. With no list set, the skip is reported on stderr and in `--json`, and a clean extension exits 0.
42. With a private term in a tracked path, later hits show only the term number and commit, and no path appears in the text output or `--json`.
43. While a private hit exists, a public hit shows only its rule id.
44. With a private list line that does not compile, the message gives its line number and never its text, and a missing tracked file is not named.
