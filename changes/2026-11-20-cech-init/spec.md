# `cech init`

`cech init [--source URL] [--ref TAG] [--dry-run] [--json] [-C DIR]` sets up a project for one managed marketplace: it writes `.cech/config.toml` and renders that marketplace's entries in the native files ([ADR 19](../../docs/adr/0019-per-project-plugin-enablement.md)).
It enables no plugin; `cech enable` does that.
See [issue 10](https://github.com/TrialAndTemper/cech/issues/10).

The pull request that builds it adds `cech init` to `docs/reference/cli.md` and adds a reference page for `.cech/config.toml`.

## Terms

- **Native files:** `.claude/settings.json` and `.codex/config.toml` in the project.
- **Release tag:** `vX.Y.Z` for a final release or `vX.Y.Z-rc.N` for a pre-release ([ADR 14](../../docs/adr/0014-releases-and-versions.md) item 2), where `X`, `Y`, `Z` and `N` are decimal with no leading zeros and `N` starts at 1; so `v0.01.0`, `v0.1.0-rc.0` and `v0.1.0-beta.1` are not release tags.
- **Versioned name:** the catalogue's name in a release build, such as `cech-0-1-0` for `v0.1.0` (ADR 14 item 3).
- **The marketplace's names:** its namespace, such as `cech`, and every versioned name of it.

## Acceptance criteria

### Choosing the release

1. WHEN `--source` is not given, THEN the source is `https://github.com/TrialAndTemper/cech.git`.
   Test: to be written
2. WHEN `--ref` is not given, THEN `init` picks the source's highest final release tag in SemVer order, skipping pre-releases (ADR 14 item 5); WHEN the source has no final release, THEN it exits 1 and says to pass `--ref`.
   Test: to be written
3. WHEN `--ref` is a release tag, final or pre-release, THEN `init` uses it; WHEN it is not a release tag, such as a branch or a commit, THEN `init` exits 2; WHEN the source has no such tag, THEN it exits 1.
   Test: to be written

### Reading the source

4. WHEN `init` reads the source, THEN it uses git with the machine's git configuration and fetches only the chosen tag; WHEN it cannot reach the source or fetch the tag, THEN it exits 3.
   Test: to be written
5. WHEN the catalogue at the tag's commit is not named with the tag's versioned name, THEN `init` exits 1.
   Test: to be written

### When it refuses

6. WHEN `.cech/config.toml` exists, THEN `init` exits 1 and names `cech enable` and `cech upgrade`.
   Test: to be written
7. WHEN either native file has a marketplace or plugin entry under one of the marketplace's names, THEN `init` exits 1 and names each such entry by file and key; entries of other marketplaces do not count.
   Test: to be written
8. WHEN a native file does not parse or has duplicate keys, THEN `init` exits 1 ([ADR 15](../../docs/adr/0015-the-cech-cli.md) item 3).
   Test: to be written
9. WHEN `init` exits other than 0, THEN it has changed no file.
   Test: to be written

### What it writes

10. WHEN `init` succeeds, THEN `.cech/config.toml` has one table for the marketplace and an empty list of enabled plugins:

    ```toml
    [marketplaces.cech]
    source = "https://github.com/TrialAndTemper/cech.git"
    ref = "v0.1.0"
    commit = "<40-hex commit the tag points to>"

    [enable]
    cech = []
    ```

    Test: to be written
11. WHEN `.cech/config.toml` has an unknown, misplaced, missing or invalid key, THEN its reader rejects the file and names the key ([ADR 17](../../docs/adr/0017-repository-data-and-state.md) item 6); a fixture of a full valid file stays accepted.
    Test: to be written
12. WHEN `init` succeeds, THEN each native file declares the marketplace at the tag under its versioned name and sets every plugin of the catalogue to `false` under the versioned name and the namespace (ADR 19 items 3, 4 and 6), and `init` writes nothing else: no `.cech/roles.toml` and no `skillOverrides` entry.
    Test: to be written
13. WHEN `init` writes a native file, THEN it creates the file if missing, and otherwise keeps every byte outside the keys it writes; each write is atomic and read back to check it (ADR 15 item 3).
    Golden tests cover CRLF files, comments in TOML, the file mode and symlinked files.
    Test: to be written
14. WHEN `init` relies on a harness behaviour, such as pinning a marketplace to a tag, THEN it uses the keys that harness documents, the behaviour has an entry with a probe in the harness data file ([ADR 9](../../docs/adr/0009-native-harness-features-first.md) item 2), and its code is in that harness's package (ADR 15 item 6).
    Test: to be written
15. WHEN a probe shows that a harness cannot pin a marketplace to a tag, THEN the work stops and the issue says so before anything else is built.
    Test: by hand: the pull request names the probe and its result.

### Output

16. WHEN `init` changes files, THEN it prints `created <path>` or `updated <path>` for each, in path order; WHEN `--dry-run` is given, THEN it prints `create <path>` or `update <path>` instead and writes nothing, as `cech build` does.
    WHEN `--json` is given, THEN it prints one object, whose fields the reference page sets.
    Test: to be written

## Out of scope

- Enabling plugins and their dependencies, `.cech/roles.toml` and `skillOverrides`: `cech enable`.
- Adding a second marketplace to a project.
- Importing hand-written entries from the native files.
- `cech check`, `cech setup` and `cech upgrade`.
- The release workflow and the host's tag protection.
