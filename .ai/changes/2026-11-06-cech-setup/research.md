# Research: what `cech setup` must do for Codex

`cech setup` is the per-clone command of [ADR 19](../../../docs/adr/0019-per-project-plugin-enablement.md) item 8.
This note gathers what is known about Codex's handling of a marketplace that a project declares, for the change that builds the command.
The gap itself, with its probe, is the entry `codex-project-marketplace-fetch` in `cli/internal/harnessdata/harnesses.toml`, and `scripts/probes/codex-project-marketplace-fetch` runs that probe.

## Runs

Both runs used a local `file://` git marketplace, a trusted project that declares it in `.codex/config.toml` with a `ref`, and an empty `CODEX_HOME`.

- **2026-10-21, Codex CLI 0.162.1.**
  Plugin and debug commands, and `codex exec` to start sessions, unauthenticated as well as logged in.
- **2026-11-06, Codex CLI 0.166.0.**
  Plugin and debug commands only, so no session started and no model was called.

## Findings

1. Until something fetches the marketplace, `codex plugin list` exits 1 and says the marketplace's snapshot does not load (both runs).
2. `codex plugin marketplace upgrade` fetches it at the declared `ref` and installs the plugins the project enables, and leaves the rest uninstalled (both runs).
   It reads the configuration of the current directory, so run elsewhere it finds no marketplace to upgrade.
3. In 0.162.1, starting a session fetched and installed the same way, and each new session moved the copy to the project's current `ref`.
   `codex debug prompt-input` and `codex plugin list` never fetch.
   Not checked in 0.166.0, since a session calls the API.
4. After a change to the `ref`, the old version keeps loading, with no warning, until the next fetch (both runs).
5. **The gap.**
   A plugin enabled after the fetch, at an unchanged `ref`, is never installed: `upgrade` reports every marketplace up to date, and in 0.162.1 a new session did not install it either (both runs).
   A later fetch at a new `ref` installs it along with the update.
6. Neither `upgrade` (both runs) nor a session (0.162.1) changed the user's `config.toml`.

## Ways around the gap, as seen in 0.162.1

- **Move the `ref`.**
  It works, but a project cannot move a pin just to enable a plugin.
- **Delete the snapshot's install marker in `CODEX_HOME`, then `upgrade`.**
  It works and leaves user config alone, but the marker is an internal file of Codex.
- **`codex plugin add <plugin>@<marketplace>`.**
  It installs the plugin, but writes an `enabled` entry into the user's `config.toml`, which ADR 19 item 6 rules out.

## Open question for the change

[ADR 15](../../../docs/adr/0015-the-cech-cli.md) item 2 has `cech` run a harness's own command rather than change the harness's state, and edit only documented settings.
The second way breaks the first rule and the third breaks ADR 19 item 6, so the change must find a way that keeps both, or decide in an ADR which rule gives.
Before choosing, check the newest Codex for a command or setting that reinstalls a project's enabled plugins, and whether the gap still holds: `mise run probes -- codex-project-marketplace-fetch`.
