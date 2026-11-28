# Research: what supporting Antigravity CLI takes

Antigravity CLI (`agy`) is Google's harness for the terminal; for individual Google accounts it replaced Gemini CLI.
This note gathers how `agy` handles plugins, project configuration and recorded runs, for a change that adds it as a third harness beside Claude Code and Codex.
It records what was seen, not what CECH should do; the last section lists the questions the change must answer.

## Runs

All runs used `agy` 1.5.18 on Linux, on 2026-11-27, with a scratch `HOME` for each check, so no user settings were involved.

- **Without a model.**
  `agy plugin validate`, `install`, `list` and `import`, and `agy acp`.
  Opening an ACP session (`initialize`, then `session/new`, with no prompt) lists every skill the session loaded as a slash command, so it shows what a project loads without calling a model.
  The test plugins were copies of the `acme` fixture marketplace in `cli/internal/app/testdata/build/acme/out/`.
- **With a model.**
  Ten runs of `agy -p --model gemini-4-hydrogen-low --output-format stream-json`, about 15,000 tokens each, in small fixture projects with one or two skills.
- Claude Code and Codex were run only to check that a plugin with the `agy` manifest added still loads in them.

## Findings

### Plugin layout

1. A plugin with only `.claude-plugin/plugin.json` does not load.
   `agy plugin validate` reports it as a Claude Code plugin that `plugin install` converts.
2. `plugin install` of such a folder converts it into a user-level copy, keeping skills and agents and dropping `dependencies`, `license`, `metadata` and the author's URL.
3. `agy` reads its own manifest from `plugin.json` at the plugin root, `.antigravity-plugin/plugin.json` or `.gemini-plugin/plugin.json`.
   It drops unknown top-level fields without a warning.
4. A plugin folder with both `.claude-plugin/plugin.json` and `.antigravity-plugin/plugin.json` (`name`, `description`, `version`, `author` and `license`) passes `agy plugin validate --strict`.
   Claude Code validates the same folder with `--strict`, installs it and enables it.
   Codex installs it, puts its skill in the prompt and prints no deprecation warning.
5. Skill frontmatter as CECH writes it (`name`, `description`) loads unchanged.
6. An agent whose frontmatter says `tools: Read, Grep` is skipped, because `agy` expects `tools` as a list.
   `tools: [Read, Grep]` loads, and Claude Code accepts that form too.
7. When two plugins in one session have the same name, the first one found wins.
   So a session cannot load two versions of a plugin.

### Catalogues and installing

8. `agy` cannot read a Claude Code `marketplace.json`: it expects each plugin's `source` as an object, and its own catalogues use `{"type": "archive", "url": …, "sha256": …}`.
9. `agy` knows only its two official marketplaces; no command or setting adds another.
   `agy plugin link` only builds links into those two.
   `agy plugin install` takes a folder, a `.zip`, a URL or `<plugin>@<marketplace>`.
10. `agy plugin import claude` copies every plugin installed in Claude Code into the user's `agy` settings once, and enables them.
    It converts them as in finding 2.
    It leaves the agents' `tools` strings as they are, so those agents do not load (finding 6).
    The copies do not follow later changes in Claude Code.
11. `agy`'s manifest has no `dependencies` field, so nothing installs a plugin's dependencies.

### Project configuration

12. A project can list plugin folders in `.agents/plugins.json`, which a team commits:

    ```json
    {"entries": [{"path": ".cech-store", "include_only": ["acme-prose", "acme-docker"]}]}
    ```

    `include_only` and `exclude` filter by folder name.
    The listed plugins load in that project only, and `agy plugin list` does not show them; an ACP session does.
13. An entry loads only when its path is inside the repository; a path outside it is skipped without a warning.
    A git-ignored folder inside the repository counts.
    The changelog gives the reason: protection against untrusted repositories.
    These routes to a folder outside the repository were all skipped: an absolute path, a `~/` path, an `inherits` entry naming a `plugins.json` outside the repository (absolute or `~/`), and a symbolic link inside the repository, to the folder or to one plugin.
    An absolute path does load when the session adds that folder to its workspace with `agy --add-dir`, which `agy acp` takes too.
    No setting in the user's or the project's settings file that was tried did the same.
    A `plugins.json` in the user's `~/.gemini/config/` loads plugins from any folder, with `include_only`, but in every project.
14. A plugin loaded this way needs `agy`'s own manifest (finding 3); it is not converted.
15. Whether a user-level plugin is enabled is kept in `~/.gemini/config/config.json`; the project file did not change it in these runs.

### Precedence

16. A plugin from the project's `.agents/plugins.json` wins over a user-level plugin of the same name.
17. A user's personal skill with the same name as a project plugin's skill loads beside it, giving two commands of that name.
    The project's `exclude` patterns do not filter skills from the user's configuration, so no project setting turns the personal copy off.

### Routing and recorded runs

18. `agy` follows a route in the project's root `AGENTS.md`.
    In one project with two skills, `acme-notes` ("House style for acme text") and `acme-changelog` ("Writes release notes and changelog entries for a new version"), the prompt asked for a release note:
    - with a root `AGENTS.md` routing release notes to `acme-notes`, both runs read only `acme-notes` and followed it;
    - without it, both runs read both skills and followed `acme-changelog`.

    No run read `AGENTS.md` through a tool, so `agy` must add it to the prompt itself.
    A nested `AGENTS.md` was not tried.
19. In `stream-json`, a skill's use shows as a `read_file` tool call on that skill's `SKILL.md`, with its full path; there is no event of its own.
    Each step reports its token usage, and a last `result` event carries the reply and the totals.
20. Six repeated runs of one prompt in two projects made the same tool calls each time and differed only in wording.

### Sign-in, versions and terms

21. `plugin validate`, `install`, `list`, `import` and an ACP session with no prompt work without a sign-in.
22. The sign-in is kept in the system keyring, so a run under a scratch `HOME` still calls the model.
23. `agy` can also run on a `GEMINI_API_KEY` (with `modelProvider: "gemini"` in its settings), on Application Default Credentials (`AGY_ADC_AUTH`), or through an enterprise model gateway.
    Gemini 4 Hydrogen is offered with an API key.
24. `agy --version` prints the bare version, such as `1.5.18`.
25. The updater's manifest for each platform (for Linux, `https://antigravity-cli-auto-updater-974169037036.us-central1.run.app/manifests/linux_amd64.json`) gives a versioned tarball URL and its SHA-512 checksum.
    The bucket behind it cannot be listed, so whether older versions stay downloadable is not known.
26. `agy` updates itself in the background.
    No setting or environment variable to turn that off is documented.
27. The [Google Antigravity Additional Terms](https://antigravity.google/terms), section 6, call "using third party software, tools, or services to access the Service" a breach, with suspension as a possible result.
    They do not mention scripted, headless or CI runs.
    Replies on Google's developer forum describe `agy -p` with `stream-json` as meant for scripting and CI under a person's own sign-in, and point automated pipelines to an API key; those replies are not terms.
    With a personal sign-in, `agy` sends the conversations to Google unless the user turns this off in settings; with a business sign-in, it does not.

## Where the plugins come from

`agy` reads neither CECH's catalogue (finding 8) nor any third-party marketplace (finding 9), so `cech` must fetch the enabled plugins at their pins itself.
Finding 13 leaves three places to put them:

1. **A copy in each clone.**
   `cech` copies the plugins into a git-ignored folder inside the repository, and the project commits `.agents/plugins.json` naming that folder, with `include_only`.
2. **A cache in the user's home, added with `--add-dir`.**
   No copy in the repository, but every run must pass the flag, and the project file must name the cache by an absolute path, which differs for each user, so the project cannot commit it.
   Without the flag, `agy` loads no plugins and gives no warning.
3. **A user-level `plugins.json`.**
   The plugins then load in every project, and `cech` never writes to user settings ([ADR 19](../../docs/adr/0019-per-project-plugin-enablement.md) item 6).

The change goes on with option 1.
Next to how Claude Code and Codex work under ADR 19, and as `changes/2026-11-06-cech-setup/research.md` describes for Codex:

| | Claude Code | Codex | `agy`, option 1 |
| --- | --- | --- | --- |
| The project commits | `.claude/settings.json` | `.codex/config.toml` | `.agents/plugins.json` |
| The plugin files live in | the user's Claude Code folder | `CODEX_HOME` | a git-ignored folder in each clone |
| Fetched by | the harness | the harness, with `cech setup` covering its gap | `cech setup` |
| After a pin changes | the harness fetches the new version | the old version loads until the next fetch | the old copy loads until `cech setup` runs again |
| The plain harness command loads them | yes | yes | yes |
| A personal skill of the same name | turned off through `skillOverrides` | loads beside it | loads beside it (finding 17) |
| Git worktrees | share one copy | share one copy | each need their own copy |

So option 1 works like Codex, with `cech setup` doing the fetch that `agy` cannot.
It costs a copy in every clone and worktree, and it is a workaround for a harness gap, which needs its entry and probe in the harness data file ([ADR 9](../../docs/adr/0009-native-harness-features-first.md) item 2).

## Questions for the change

- **The copy and ADR 22.**
  The copy of option 1 is a project copy of managed skills, which `cech check` rejects under [ADR 22](../../docs/adr/0022-two-install-paths.md) item 5.
  Item 5 makes no difference between a copy made by hand and a generated, git-ignored copy that `cech setup` refreshes, so the change must settle, by ADR, whether the second is allowed.
- **Generated files.**
  `cech build` would add `.antigravity-plugin/plugin.json` to each plugin (finding 4), and agents need `tools` as a list (finding 6).
  [ADR 10](../../docs/adr/0010-claude-code-and-codex-from-one-source.md) item 3 allows generated extras for Codex only, and item 4 says a skill's claims hold on both harnesses, so both items need changing.
- **The ADRs written for two harnesses.**
  ADR 19 items 2 and 5, ADR 22 item 6 and ADR 25 item 6 name Claude Code and Codex; each needs a new or changed item.
- **Personal copies.**
  As in Codex, nothing in a project turns off a personal skill of the same name (finding 17).
  That is a gap to record in the harness data file, as ADR 22 item 6 does for Codex.
- **Pins and versions.**
  A session loads one plugin of each name (finding 7), so a project pins one version of each plugin.
  Each project has its own copy inside its repository (finding 13), so two projects can still pin different versions.
  `cech` resolves dependencies itself (finding 11), as ADR 19 item 5 already says.
- **The tool pin.**
  A mise pin could use the versioned tarball and checksum (finding 25), but only if old versions stay downloadable and the background update (finding 26) leaves a pinned binary alone.
  Check both after the next release: request the 1.5.18 tarball URL again, and see whether a mise-installed `agy` changes after an update.
- **Evals.**
  The eval runner could read a skill's use from `stream-json` (finding 19).
  Without a route, `agy` reads several candidate skills before following one (finding 18), so the runner needs a rule for which read counts as the skill's use.
  Recorded runs in CI would use a `GEMINI_API_KEY` rather than a person's sign-in (findings 23 and 27).
