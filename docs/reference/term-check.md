# The term check

The term check keeps content specific to a private organisation out of CECH ([ADR 4](../adr/0004-vendor-neutrality.md)).
It is the `terms` gate, run by `scripts/check-terms` through `mise run terms`, and it matches with the gitleaks pinned in `mise.toml`.
How to set up a private list, and what to do with a false positive, is in [CONTRIBUTING.md](../../CONTRIBUTING.md#keep-cech-vendor-neutral).

## Tiers

| Tier | Source | What it flags |
| --- | --- | --- |
| Generic rules | `checks/terms/rules.toml`, plus gitleaks' own secret rules | Shapes that suggest content copied from a private environment: email addresses outside the allowlist, cloud account and resource ids, signed or shared-access URLs and single-sign-on user formats |
| Message rules | `checks/terms/messages.toml` | In commit and tag messages and ref names only: issue keys from other trackers, since CECH refers to its own issues as `#N` |
| Vendor names | `checks/terms/names.txt` | Product and brand names CECH does not cover, since such a name hints at whose private stack the content came from |
| Private terms | The file named by `CECH_PRIVATE_TERMS`, by default `.private/denylist.txt`, which git ignores | Terms a contributor or maintainer must never let reach CECH |

The vendor names list matches whole words, ignoring case, with spaces, `-` and `_` between words optional and interchangeable.
Only listed names are flagged; a skill about a public product drops the name from the list.

The private list holds one case-insensitive RE2 regular expression per line, with `#` starting a comment line; a line may not contain `'''`.

## What it scans

| Mode | Scans | Used by |
| --- | --- | --- |
| none | The staged tree, ref names and tag messages, and the commits in `CECH_RANGE` if it is set, else those missing from `origin/main`, or every commit reachable from `HEAD` when `origin/main` is absent | CI |
| `--staged` | The staged tree | The `pre-commit` hook |
| `--message FILE` | One commit message | The `commit-msg` hook |
| `--range RANGE` | The staged tree, ref names, tag messages, and the diffs, paths and messages of the commits in the range | A contributor, by hand |
| `--all` | The same for every commit reachable from any ref | A contributor, by hand |

CI runs the private tier on pushes to `main` and on pull requests from branches in the repository, from a repository secret, and fails there when the secret is empty or missing.
Pull requests from forks run only the public tiers.
Without a private list, the private tier is skipped; `CECH_REQUIRE_PRIVATE_TERMS=1` makes a missing list an error.

## Exceptions

- A false positive on a public tier gets an entry in the tier's allowlist in `checks/terms/`, or the marker `cech-terms: allow <reason>` on the same line.
- A `.gitleaksignore` file anywhere in the tree stops the check: exceptions live in `checks/terms/` or the private list.
- Allowlists and the marker do not apply to the private tier; a false positive there is fixed in the list itself.

## Output

- A private-tier hit names the file, the line and the term's line number in the list, never the term.
- If a file path holds a private term, later private-tier hits print only their rule and commit.
- While the private tier has hits, public hits print only their rule, since their text or path could hold a private term.
- Exit status: 0 clean, 1 hits found, 2 the check could not run.
