# Code owners gate

Implemented 2026-11-20; not maintained: the code and its tests are current.

A gate, `scripts/check-codeowners`, run as `mise run lint:codeowners` and so by `mise run check:os`, fails on the code owners errors that GitHub reports, such as an owner without write access; [CONTRIBUTING.md](../../../CONTRIBUTING.md#for-maintainers) says what happens then.
GitHub answers a public repository without a token too, but the gate uses one, which raises the rate limit on shared runners and reads a private repository, so it follows ADR 5's rule for gates that need a secret.
[Issue #32](https://github.com/TrialAndTemper/cech/issues/32) asks for it.

## Acceptance criteria

### When the gate runs

1. WHEN `GITHUB_ACTIONS` is `true` and `GITHUB_TOKEN`, `GITHUB_REPOSITORY` or `GITHUB_SHA` is missing or empty, THEN the gate exits 2.
   WHEN `GITHUB_ACTIONS` is not `true` and any of the three is missing or empty, THEN the gate prints that it was skipped and exits 0.
   Test: `test/codeowners.bats` "a missing or empty token in GitHub Actions is an error, not a skip"; by hand: the skip, with none of the three set.
2. WHEN `GITHUB_TOKEN` holds a `"`, a `\` or a line break, `GITHUB_REPOSITORY` is not `owner/name`, has a `.` or `..` part, or holds a character other than letters, digits, `.`, `_`, `-` and `/`, or `GITHUB_SHA` holds anything but lower-case hex digits, or git, curl or jq is missing, THEN the gate exits 2.
   Test: by hand: a token with `"`, one with `\`, one with a line break, `acme/../x`, `acme/sk ills`, `HEAD`, and a `PATH` without git, then without curl, then without jq.
3. The gate takes the first of `.github/CODEOWNERS`, `CODEOWNERS` and `docs/CODEOWNERS` that exists in the commit, as GitHub does.
   WHEN the commit has none, THEN the gate exits 0 without asking GitHub, which would answer 404.
   WHEN the commit is missing from the clone, or git cannot list its files, THEN the gate exits 2.
   Test: `test/codeowners.bats` "a code owners file at the root or in docs/ is checked too", "a commit missing from the clone is an error, not a commit without the file"; by hand: a commit with no file, and reading the script for a failed `git ls-tree`.

### The request

4. WHEN the commit has a code owners file, THEN the gate sends `GET {GITHUB_API_URL}/repos/{owner}/{name}/codeowners/errors?ref={commit}` with curl and `X-GitHub-Api-Version: 2022-11-28`, where `GITHUB_API_URL` defaults to `https://api.github.com`; curl ignores the user's `.curlrc` and reads the token on standard input, never from the command line.
   Test: `test/codeowners.bats` "the request names the commit under test"; by hand: reading the script for `.curlrc` and the token, and in a scratch repository for the answer GitHub gives for a pull request's merge commit and a merge queue commit ([research](research/api-probe.md)).
5. WHEN the request fails, GitHub answers other than 200, or a 200 answer is not JSON or its `errors` is not a list, THEN the gate exits 2.
   A failed request names the API URL; an answer other than 200 shows its status and the first lines of its body.
   Test: `test/codeowners.bats` "a 404 for a commit with a code owners file is an error, not a pass", "a 200 answer without a list of errors is an error"; by hand: an unreachable host, a bad token (401), an unknown commit (500) and an answer that is not JSON.

### Findings

6. WHEN the list holds errors, THEN the gate prints each as `codeowners: <path>:<line>: <message>`, then a line naming the file and their count, and exits 1; WHEN the list is empty, THEN it exits 0.
   The message is cut to its first line; a missing path or line is `?`, a missing or empty message is the error's `kind`, or `error` when both are missing or empty, and an item that is not an object prints `?:?: error`.
   Test: `test/codeowners.bats` "each reported error fails the gate and names its line", "the request names the commit under test"; by hand: the other forms, GitHub's own messages in the scratch repository, and an empty list against this repository's `main`.

### The command line

7. WHEN the gate gets `-h` or `--help` alone, THEN it prints the usage and exits 0; WHEN it gets any other argument, an empty one included, or more than one, THEN it exits 2.
   Test: by hand: `--help`, `-h x`, `''` and `x`.

## Out of scope

- Checking owners without GitHub: only GitHub knows who has write access, so neither `cech build` nor `cech validate` can.
