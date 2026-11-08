# Evidence moved out of the ADRs

The ADR restructure kept one sentence per finding in each ADR ([ADR 24](../../../docs/adr/0024-where-each-kind-of-text-lives.md) item 3).
This note holds the detail those sentences summarise, as the ADRs stated it.

## ADR 16: building the `cech` CLI

- Checked on 2026-10-27 with Go 1.27.3: a module in a subdirectory takes its versions from tags prefixed with that directory, so the module in `cli/` is versioned by `cli/vX.Y.Z` tags, and `go install` of a version records it in the binary's build information.
- Checked on 2026-10-31 with Go 1.27.3: of the modules `github.com/spf13/cobra` v1.10.2 (Apache-2.0) requires, only two are built into the CLI: `github.com/spf13/pflag` v1.0.10 (BSD-3-Clause) and `github.com/inconshreveable/mousetrap` v1.1.0 (Apache-2.0), which only Windows builds use.
