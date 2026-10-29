# Harness probe job

Implemented 2026-10-30; not maintained: the code and its tests are current.

The scheduled job of [ADR 9](../../../../docs/adr/0009-native-harness-features-first.md) item 4, and the check of the newest Codex that [ADR 10](../../../../docs/adr/0010-claude-code-and-codex-from-one-source.md) item 5 asks of it.

## The data file

1. An entry of `cli/internal/harnessdata/harnesses.toml` may have the key `unattended`, a boolean, `false` when absent.
   Any other type is a problem that names the entry and the key, as `<entry>: unattended is a string, not a boolean`.
2. An entry marked `unattended = true` has an executable POSIX sh script `scripts/probes/<entry name>`, and each script there has such an entry.
3. The prose `probe` stays as the steps a person reads; the script carries out the same steps.

## A probe script

4. It sources `scripts/lib/common.sh`, then `scripts/lib/probe.sh`, and sets `PROG` to its entry's name.
5. It runs, on Linux and macOS, the harness with a new, empty home and config folder (`CLAUDE_CONFIG_DIR` or `CODEX_HOME`) as its `HOME`, and no environment but `PATH` and `TMPDIR`, and no stdin, so no login, key or setting reaches it and it cannot wait for input.
6. It exits 0 on the result in the entry's `expect`, after printing `<entry>: ok`.
7. On any other result it exits 1, after printing `<entry>: <what differs>` and the harness output it checked, indented, to stderr.
   A harness command that fails where the entry expects success is such a result.
8. When it cannot run, such as with no harness on `PATH`, it exits 2 with `<entry>: <reason>` on stderr.
9. `codex-plugin-metadata` also exits 1 if any Codex plugin command it ran printed `deprecat` in any case; of `codex debug prompt-input` it searches only stderr, since stdout holds Codex's own prompt and tool schemas.

## The runner

10. `scripts/run-probes [--results FILE] [ENTRY...]`, also `mise run probes`, runs the script of each named entry, or of every file in `scripts/probes/`, in name order.
    It is not in `check`.
11. Before each it prints `probes: <entry> on <harness> <version>`, the harness being `claude-code` or `codex` from the start of the name, and the version the first `X.Y.Z` in `claude --version` or `codex --version`, `unknown` if none, `missing` with no command.
12. With `--results FILE`, it writes one line per probe: `<entry> <result> <harness> <version>`, the result `ok`, `differs` (exit 1) or `failed` (any other).
13. It exits 0 if every probe exited 0; 2 if any exited other than 0 or 1, or the runner itself could not run; 1 otherwise.
    Before a non-zero exit it lists the entries that differ, then those that could not run, on stderr.

## The report

14. `scripts/report-probes RESULTS RUN_URL` reads the runner's results file; `gh` takes the repository from `GH_REPO`.
    A line whose entry has characters other than `a-z`, `0-9` and `-`, or no script in `scripts/probes/`, counts as a probe that could not run.
15. Each `differs` line comments on the open issue titled `harness probe: <entry> gives another result`, or opens one labelled `triage`, `bug` and `portability`.
    The body names the entry, the harness and version, the run, and what to do next.
16. A `failed` line, a script in `scripts/probes/` with no line, as when the step's timeout stopped the run, or a file with no `differs` line, comments on the open issue titled `harness probes: the scheduled job fails on main`, or opens one labelled `triage`, `bug` and `ci`.
    Its body names the run and any probes that could not run or gave no result.
17. An open issue is found by its exact title in the plain list of up to 500 open issues, not by search.

## The workflow

18. `.github/workflows/probes.yml` runs daily, on demand, and on pull requests and pushes to `main` that touch `scripts/run-probes`, `scripts/report-probes`, `scripts/probes/`, `scripts/lib/`, the data file or the workflow.
19. Its `probes` job runs on `ubuntu-24.04`, installs `@anthropic-ai/claude-code@latest` and `@openai/codex@latest` with npm on the newest Node LTS, with a step timeout of 5 minutes, and runs the runner, with a step timeout of 10 minutes and a job timeout above both, and `TMPDIR` under the runner's temporary folder, since Codex warns when its home is under `/tmp`.
    It passes the results to the `report` job even when a probe fails.
20. The `report` job runs only when the `probes` job failed on `main`, outside a pull request, and only it may write issues.
    Runs on `main` wait for one another rather than run at once; a newer waiting run replaces an older one.
21. The workflow is not a required check, so it never blocks a merge.
