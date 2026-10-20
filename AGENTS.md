# Agent operating contract

## Where am I?

The root of CECH, a marketplace of agent skills for Claude Code and Codex that organisations extend rather than fork ([README](README.md)).
It has no skills yet: the tree holds the decision records, a first `cech` CLI that only prints its version, the gates that check every change, and their tests.
This is the only `AGENTS.md`, so it answers for every path.

## What belongs to this layer?

Everything tracked in the repository.
People and agents follow the same rules: [CONTRIBUTING.md](CONTRIBUTING.md) and the ADRs state them, and this file routes to them and adds the few that matter most to agents.

## Local boundaries

A lower `AGENTS.md`, once one exists, adds or narrows rules for its own paths and never weakens one from this file.

- The ADRs in `docs/adr/` are the contracts.
  Read the ones a change touches before designing it; changing a contract needs a new ADR or an amendment note ([ADR 1](docs/adr/0001-record-architecture-decisions.md)).
- Keep files, commit messages, and branch and tag names vendor-neutral ([ADR 4](docs/adr/0004-vendor-neutrality.md)): nothing specific to a private organisation, and invented examples such as `example.com` and `acme`.
  Never copy text from git-ignored notes into the tree; rewrite it, run `mise run terms`, and let a person read it.
- Text from another project needs a compatible licence, a `NOTICE` entry and, in a skill, a source note naming the upstream project and commit ([ADR 2](docs/adr/0002-licence-and-contributions.md)).
- ADR evidence cites tree files, dates and tool versions, never pull request or issue numbers, so each ADR stands alone.
- Commit messages and other published text link only to public, lasting pages, never to an agent session or a page that needs a login ([ADR 11](docs/adr/0011-change-record.md)).
- Push, open a pull request or merge only when a person asks.

## Routing

| Change | Destination | Rule or reference |
| --- | --- | --- |
| A new decision that is costly to reverse, also added to `docs/adr/README.md` | `docs/adr/` | [ADR 1](docs/adr/0001-record-architecture-decisions.md) |
| A gate's behaviour, with its task in `mise.toml` and tests in `test/` | `scripts/` | [ADR 10](docs/adr/0010-gates-and-ci.md) |
| A CLI command or package, with its Go tests | `cli/` | [ADR 14](docs/adr/0014-the-cech-cli.md) |
| A Go dependency, in the same change as an ADR 14 amendment | `cli/go.mod` | [ADR 14](docs/adr/0014-the-cech-cli.md) item 8 |
| A public term rule or an allowlisted email address | `checks/terms/` | [ADR 4](docs/adr/0004-vendor-neutrality.md) |
| A tool pin or a task, then `mise lock` for a pin | `mise.toml` | [ADR 10](docs/adr/0010-gates-and-ci.md) item 6 |
| A CI step, kept in step with the tasks `check` depends on | `.github/workflows/ci.yml` | [ADR 10](docs/adr/0010-gates-and-ci.md) items 1 and 7 |
| A rule for contributors; route agents to it from here | `CONTRIBUTING.md` | [ADR 10](docs/adr/0010-gates-and-ci.md) item 2 |

## Direct ownership

| Path | Responsibility |
| --- | --- |
| `docs/` | Decision records; an accepted ADR changes only by an amendment note or a superseding ADR, apart from typo and link fixes |
| `scripts/` | Gate scripts in POSIX sh that run under dash, macOS sh and Git Bash; `scripts/lib/` holds the helpers they source, `scripts/hooks/` the optional git hooks |
| `checks/` | Public tiers of the term check; the private tier lives outside git |
| `cli/` | The `cech` CLI, Go module `github.com/TrialAndTemper/cech/cli`: `cmd/cech` for `main`, `internal/` for the rest, golden files in `testdata/` |
| `test/` | bats suites for the scripts and tasks; a script change comes with tests here |
| `mise.toml` | Exact tool pins and every task; the single entry point for the gates |
| `mise.lock` | Downloads and checksums for each pin and platform, written by `mise lock` |
| `.github/` | The CI workflow, issue forms and pull request template |
| `.rumdl.toml` | Markdown lint and format settings: sentence per line, no line limit |
| `lychee.toml` | Link check settings |
| `.shellcheckrc` | shellcheck settings for every shell file |
| `.editorconfig` | Encoding, indentation and the shfmt dialect per path |
| `.gitattributes` | LF line endings for every text file, which the scripts need on Windows |
| `.gitignore` | Keeps private notes, local overrides and tool caches out of git |
| `README.md` | What CECH is and its status; no install steps until there is something to install |
| `CONTRIBUTING.md` | The full contributor rules this file routes to |
| `SECURITY.md` | How to report a vulnerability privately |
| `CODE_OF_CONDUCT.md` | Adapted Contributor Covenant under CC BY-SA 4.0; edit only its CECH-specific parts |
| `LICENSE` | MIT licence |
| `NOTICE` | Notices of third-party material, one entry per copied or adapted text |

## How to work

mise is the task runner: use `mise run …` rather than the tools it pins, and run `mise tasks` before inventing a command.

- **Verification:** `mise run check` runs every gate, as CI does; `mise run <gate>` runs one, and `mise run format:fix` rewrites files into their checked format.
- **Trust:** after any change to `mise.toml`, run `mise trust`; with mise's `paranoid` setting the change revokes trust, and every mise tool then fails.
- **Per-commit checks:** check the commits out one by one in a separate `git worktree` and run `mise run check` there; under `git rebase --exec` the suites run inside a git process whose environment can point them at the real repository.
- **Markdown:** one sentence per line, never wrapped ([ADR 13](docs/adr/0013-markdown-source-format.md)).
- **Commits:** follow [CONTRIBUTING.md](CONTRIBUTING.md#commits); `mise run commits` checks them.
  Sign off with `git commit -s`, credit yourself with a `Co-authored-by:` trailer, and fold review fixes in with `git commit -s --fixup=<commit>` and `git rebase --autosquash origin/main`.
- **Issue tracker:** GitHub issues on `TrialAndTemper/cech`, labelled as [CONTRIBUTING.md](CONTRIBUTING.md#issues) describes; questions go to Discussions.

## Self-update and layer growth

Keep this file on five questions: where an agent is, what the layer owns, which boundaries hold, where a change goes, and how to do and verify it.
Update the routing and ownership tables in the same change that adds, moves or removes a path.
Put a recurring instruction in the layer that owns it, and point to `CONTRIBUTING.md` or an ADR rather than repeating it.
Past 100 lines, split: give a directory with its own boundary, routing or work method its own `AGENTS.md`, move its answers there, and route to it from here.
