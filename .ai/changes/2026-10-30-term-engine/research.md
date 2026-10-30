# Term engine research

Measurements behind [ADR 24](../../../docs/adr/0024-the-term-check-engine.md), taken on 2026-10-29 and 2026-10-30 on Linux x86-64 with Go 1.27.3.
Each spike was a small `main` package in a scratch module, built with `CGO_ENABLED=0 go build -trimpath -ldflags='-s -w'`.
"Modules" counts the third-party modules linked into the binary (`go list -deps`), not every line of `go.mod`.

## Candidates

| Engine | Version and date | Licence | Modules | Binary |
| --- | --- | --- | --- | --- |
| None: today's `cech` | | | 1 | 2.6 MB |
| gitleaks as a module | 8.30.1, 2026-03-21 | MIT | 62 | 9.4 MB |
| Betterleaks v1 as a module | 1.9.0, 2026-09-29 | MIT | 49 | 17.7 MB |
| Betterleaks v2 as a module | 2.0.0-rc.5, 2026-10-28 | MIT | 47 | 18.8 MB |
| A standard-library matcher | | | 1 | about 2.6 MB |

## Findings

- gitleaks' README opens with a notice that the project is feature-complete, that later releases are security fixes only, and that its author now works on Betterleaks.
  Its `detect` package works as a library, but the README documents no library use.
  Among its linked modules are archive and compression libraries, a configuration framework and a template library.
- gitleaks' default config has 222 rules, using regex, keywords, entropy (130 rules), one secret group, and allowlists by regex, stopword and path.
  Betterleaks' v2 default config has 473 rules, 427 of them with expression filters and 236 with provider validation programs, so it cannot run on a standard-library matcher without an expression interpreter.
- Betterleaks v1.9.0 loaded today's `checks/terms/rules.toml` unchanged.
  It flagged an address not in the allowlist and a GitHub token, hid a line carrying `cech-terms: allow`, and returned an error, not a crash, for a regex that does not compile.
  Its default config has 463 rules.
  A child file with `[extend] path` to a copy of the generic rules, plus a global allowlist of one address, hid that address and still flagged another; the path resolved against the working directory, not the child file.
  Run over every tracked file outside `checks/terms/`, it found nothing.
- Betterleaks v1.9.0's detector constructor exits the process through its logger when the validation expressions in a config fail to compile; parts of its library API are marked deprecated, to be removed in v2.
- Betterleaks v2.0.0-rc.5 rejected `checks/terms/rules.toml` in strict mode: every `allowlists` table is an unknown field.
  Its migration guide maps each allowlist to an expression `filter` or `prefilter`, and `entropy` to a filter.
- Betterleaks still ships maintenance releases of v1.
  Its v1 CLI has every option `scripts/check-terms` passes to gitleaks: `-c`, `--no-banner`, `--exit-code`, `--ignore-gitleaks-allow` (which drops both `betterleaks:allow` and `gitleaks:allow`), `-i`, `-f template`, `--report-template`, `-r`, `-l` and `--log-opts`.
  It reads `.betterleaksignore`, and `.gitleaksignore` for backward compatibility.
  mise's registry has `betterleaks` through `aqua:betterleaks/betterleaks`, with 1.9.0 available; the 1.9.0 release has builds for Linux, macOS and Windows on x86-64 and arm64, and a checksum file.
