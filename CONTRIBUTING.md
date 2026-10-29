# Contributing to CECH

Thank you for helping.
This guide covers set-up, the gates, writing, commits, issues, pull requests and keeping CECH vendor-neutral.
Everyone taking part follows the [code of conduct](CODE_OF_CONDUCT.md).
Report security problems privately, as [SECURITY.md](SECURITY.md) explains, not in an issue.

## Before you start

The [architecture decision records](docs/adr/README.md) (ADRs) hold the contracts CECH follows and the reasons for them.
A change that fits an accepted ADR needs no new one.
A change to a contract, or one that adds repo-wide machinery such as a command, a hook or a file format, needs a new ADR or a change to an accepted one ([ADR 1](docs/adr/0001-record-architecture-decisions.md)).
An ADR holds the decision and its reasons; detail such as keys, flags and messages goes in a spec, as [.ai/README.md](.ai/README.md) explains.
For a larger change, open an issue first so we can agree on the approach.
Ask questions in [Discussions](https://github.com/TrialAndTemper/cech/discussions); issues are for bugs and proposals.

By contributing you agree that your work is licensed under the licence of the files it adds or changes: MIT unless a file or folder says otherwise ([ADR 2](docs/adr/0002-licence-and-contributions.md)).
Text copied or adapted from another project needs a compatible licence, an entry in [NOTICE](NOTICE) and, in a skill, a source note naming the upstream project and the commit it came from; rewrite it where you can.
A skill based on a person's unpublished work ships only once they agree; credit published work by citing it.

## Set up

You need git, and [mise](https://mise.jdx.dev/getting-started.html) 2026.10.3 or later.
mise installs the exact tool versions pinned in `mise.toml` and runs the gates as tasks ([ADR 10](docs/adr/0010-gates-and-ci.md)).
On Windows, use Git Bash: the tasks run with `sh`.

In a new clone, trust the repository's `mise.toml` and install the tools:

```sh
mise trust
mise install
```

mise reads `mise.toml` only after `mise trust`.
If your mise runs with its `paranoid` setting, every change to `mise.toml` takes the trust away again, including a change from a pull, checkout or rebase.
Run `mise trust` after such a change.
Until then mise refuses the file, and every tool it provides, even one from your global config, fails with an error about untrusted config files.

## Run the gates

```sh
mise run check
```

This runs every gate, as CI does.
`mise run <gate>` runs one, and `mise tasks` lists every task:

| Task | Checks |
| --- | --- |
| `commits` | commit message format and sign-off |
| `terms` | names and data that must not be in the repository |
| `lint` | Markdown, shell scripts, workflows and the CLI's Go code |
| `format:check` | formatting of Markdown, shell scripts and Go code |
| `test` | the tests of the CLI, and of the gate scripts (not on Windows) |
| `docs:check` | links in the Markdown, YAML and text files; needs the network |

`mise run format:fix` rewrites the files the format check would change.

The link check also reads hidden directories such as `.github/`.
GitHub rate-limits anonymous requests and hides private repositories from them, so links to github.com may fail without a token.
lychee sends `GITHUB_TOKEN`, if set, only to GitHub:

```sh
GITHUB_TOKEN=$(gh auth token) mise run docs:check
```

With a token, lychee checks only that a GitHub repository exists, not the file or heading a link names.

CI runs `mise run check` on every pull request and every push to `main`, on Linux, macOS and Windows, and the private term tier where its secret reaches.
A new gate goes into `check` in `mise.toml`; the workflow needs no change.
A pull request merges only when CI passes.

A test that fails only sometimes is not fixed by a green rerun: open an issue for it, then fix it, or skip it with a reason that names the issue.
A skipped test of a gate script leaves a way for that gate to pass when it should fail ([ADR 10](docs/adr/0010-gates-and-ci.md) item 5), so fix it before the next change to that gate.

### The CLI's Go gates

The `cech` CLI is a Go module in `cli/` ([ADR 14](docs/adr/0014-the-cech-cli.md)).
mise installs the pinned Go, golangci-lint and govulncheck, and `scripts/check-go` runs them in `cli/` with the pinned Go only:

| Task | Runs |
| --- | --- |
| `lint:go` | `go vet`, golangci-lint with the linters in `cli/.golangci.yml`, and `go mod tidy -diff` |
| `format:check:go` | the diff that gofumpt and goimports would apply; `format:fix:go` applies it |
| `test:go` | `go test`, with the race detector where the platform has one, cgo is on (it needs a C compiler) and the race runtime builds |
| `vuln` | govulncheck: known vulnerabilities in the CLI and its Go release; needs the network |

Golden files in `cli/internal/app/testdata/` hold the exact output of each command.
After an intended change to the output, rewrite them and review the diff:

```sh
mise -C cli exec -- go test ./internal/app -update
```

`check` leaves out `vuln`, since a new report in its database can fail a branch that has not changed.
CI runs it on pull requests that touch the CLI or its tools, without blocking them, and daily on `main`, where a failure opens an issue.

A new Go dependency needs a change to ADR 14 in the same pull request.

### Optional git hooks

The hooks in `scripts/hooks/` run some of the gates before git records a commit:

```sh
git config core.hooksPath scripts/hooks
```

`pre-commit` runs the term check, lint and format checks on what is staged, and `commit-msg` checks the message; neither runs the tests or the checks that need the network.
They save a round trip to CI, but they are a convenience: CI is the authority, and no rule lives only in a hook.

## Writing

- Markdown has one sentence per line, never wrapped; shorten a long sentence rather than break it ([ADR 13](docs/adr/0013-markdown-source-format.md)).
  `mise run format:fix` reflows it.
- Refer to Claude Code and Codex as harnesses, the programs that run an agent ([ADR 3](docs/adr/0003-a-marketplace-you-extend-not-fork.md)).
  "Tool" means something else: a tool pinned in `mise.toml`, the `tool` plugin category, a tool an agent calls, or a command-line program.
- Shell scripts are POSIX sh that runs under dash, macOS sh and Git Bash, and pass shellcheck and shfmt.
  A change to a script comes with a test in `test/` for each way it could let a gate pass when it should fail, where review would likely miss it, such as after a tool bump, on another platform or on the staged tree; a comment on each case names the way it guards ([ADR 10](docs/adr/0010-gates-and-ci.md) item 5).
  Check any other way by hand, and say so in the pull request.
- Keep an ADR under 800 words of prose outside code blocks ([ADR 1](docs/adr/0001-record-architecture-decisions.md) item 6).
- Until the first plugin merges, an accepted ADR is edited in place, and the commit says why.
  From then on, it is not rewritten, apart from typo and link fixes: a refinement goes in a dated amendment note, and a different choice in a new ADR that supersedes it ([ADR 1](docs/adr/0001-record-architecture-decisions.md)).

## Commits

Commits are the record of the project ([ADR 11](docs/adr/0011-change-record.md)): reviewers read each one, and the reason for a change belongs in its message, not only in the pull request.

- Make each commit one logical change that leaves the gates passing.
- Write the header in the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) form `type(scope): summary`, at most 72 characters, imperative, with no final full stop.
  Types: `feat`, `fix`, `docs`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.
  The scope is a plugin name or an area such as `cli`, `adr`, `ci` or `checks`, in lower-case words joined by `-`; leave it out when a change spans areas.
  A revert uses `revert:` and the reverted header, not git's default `Revert "…"`.
- Leave a blank line after the header.
  In the body, say why, wrapped at 72 characters.
- Mark a breaking change with `!` after the scope and a `BREAKING CHANGE:` footer saying what users must do.
- Add a `Co-authored-by:` trailer for each other author, human or tool.
- Link only to public pages that last, such as a CECH issue, a spec or an upstream commit.

### Sign-off

Every commit carries a `Signed-off-by:` trailer with your name and the email git records as the commit's author.
It states that you may submit the work under its licence ([Developer Certificate of Origin](https://developercertificate.org/)).
`git commit -s` adds it.

The term check flags email addresses that are not on its allowlist, including those in sign-off and `Co-authored-by:` trailers.
Either set your email for this repository to your GitHub no-reply address:

```sh
git config user.email "<id>+<user>@users.noreply.github.com"
```

or add each such address, in the same pull request, to the allowlist of authors and sign-offs under the `email` rule in `checks/terms/rules.toml`.
Please use a personal address, not one from your employer.

## Issues

A new issue gets `triage` and its kind, `bug` or `proposal`, from its form.
A maintainer then gives it one status label at a time, replacing the last:

| Label | Meaning |
| --- | --- |
| `triage` | New; not looked at yet |
| `backlog` | Accepted, but not ready to start |
| `todo` | Ready: someone could start without more questions |
| `in progress` | Someone owns it and is working on it |

When an issue is finished, a maintainer removes its status label and closes it with GitHub's reason (completed, not planned or duplicate) and a comment saying what was done or which issue it duplicates.

A maintainer may add area labels beside the status, one for each part of the repository the issue touches:

| Label | Area |
| --- | --- |
| `documentation` | The docs, ADRs or guides |
| `cli` | The `cech` CLI in `cli/` |
| `gates` | Gate scripts in `scripts/`, mise tasks and the term check in `checks/` |
| `tests` | Work on the tests themselves, such as gaps, flaky cases or the bats helpers in `test/`; a test for a change in another area takes that area's label |
| `ci` | CI and scheduled workflows in `.github/workflows/` |
| `dependencies` | Tool pins in `mise.toml` and Go modules in `cli/go.mod` |
| `portability` | Behaviour that differs by operating system, shell or harness |
| `skills` | Skills and plugins in the marketplace |

A maintainer may also add `security` for a public security finding, such as a vulnerable dependency, `accessibility` for a barrier for people with disabilities, `good first issue` for an issue suited to a newcomer, and `help wanted` for one where outside help is welcome.
A vulnerability report itself goes through [SECURITY.md](SECURITY.md), not an issue.

## Pull requests

- Rebase your branch on the latest `main` rather than merging `main` into it.
- Push review fixes as fixup commits, aimed at the commit they fix:

  ```sh
  git commit -s --fixup=<commit>
  ```

- Before merge, fold them in and update your branch:

  ```sh
  git fetch origin
  git rebase --autosquash origin/main
  git push --force-with-lease
  ```

  The commit gate fails while any `fixup!`, `squash!` or `amend!` commit remains.
- Pull requests are merged by rebase only, so `main` keeps your commits as they are, in a straight line.

## Keep CECH vendor-neutral

CECH holds nothing specific to a private organisation: no internal names, hosts, emails, products, ids or quotes in files, commit messages, or branch and tag names ([ADR 4](docs/adr/0004-vendor-neutrality.md)).
Examples use invented data such as `example.com`, `*.example`, `*.test` and the company `acme`.

The term check enforces this with rules in `checks/terms/`.
For a false positive from a public rule, add an allowlist entry there, or put `cech-terms: allow <reason>` on the same line.

### A private term list

If you work with private material, keep your own list of terms that must never reach CECH.
The term check reads it from the file named by `CECH_PRIVATE_TERMS`, or by default from `.private/denylist.txt`, which git ignores:

```sh
CECH_PRIVATE_TERMS=~/private-terms.txt mise run terms
```

Put one case-insensitive regular expression in Go's RE2 syntax on each line; a line that starts with `#` is a comment, and a line may not contain `'''`.
A hit names the file, the line and the term's line number in the list, never the term.
The allowlists and the inline marker do not apply to this tier: fix a false positive in the list itself, for example with a narrower expression.
Without a list this tier is skipped; set `CECH_REQUIRE_PRIVATE_TERMS=1` to make a missing list an error.

### For maintainers

CI reads the private list from the repository secret `PRIVATE_TERMS`, one regular expression per line.
It uses the secret on pushes to `main` and on pull requests from branches in this repository, and there an empty or missing secret fails the run.
Pull requests from forks never receive it and run only the public rules.
To replace the secret from your local list:

```sh
gh secret set PRIVATE_TERMS --repo TrialAndTemper/cech < .private/denylist.txt
```
