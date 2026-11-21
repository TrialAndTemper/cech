# 25. Evals

- Status: accepted
- Date: 2026-10-29
- Deciders: Michał Bożek

## Context

A skill's claims must hold on both harnesses ([ADR 10](0010-claude-code-and-codex-from-one-source.md) item 4).
An agent learns of a skill from its description and from routing rows in `AGENTS.md` files.
A task the skill should not change shows up a runner or grader that credits the skill with an effect it does not have.

Checked on 2026-10-29 with Claude Code 2.1.312 and Codex CLI 0.165.0:

- `claude plugin eval` runs on Claude Code only and loads no project instructions.
- Claude Code reports a skill load as a `Skill` call; Codex shows a load only as the skill's text in a command's output.
  Codex can read files outside its workspace.
- Both read the root `AGENTS.md` at start; Claude Code reads a nested one only after reading a file beside it, and Codex only when started in its folder or below.
- In probes of one skill, prompts avoiding the description's words loaded it in 6 of 9 runs on Claude Code and 3 of 9 on Codex; with a routing row in the root `AGENTS.md`, 9 and 4 of 9.
  Prompts using its words loaded it almost always.

## Decision

1. **Three families of case.**
   A *content* case tests one rule of the skill, its *claim*.
   A *control* case gives a task the skill should not change.
   Both run in two *arms*, with and without the plugin.
   Both arms are *routed*: the root `AGENTS.md` routes to the skill.
   A *trigger* case checks whether one prompt loads the skill.
   It runs with the plugin, routed and *unrouted*, without that route.
2. **Evals sit beside the plugins.**
   A plugin's eval folder, `evals/<category>/<plugin>/` ([ADR 12](0012-categories-and-names.md) item 2, [ADR 13](0013-attribution.md) item 4), holds:
   - a fixture project that every run starts in, with no route to the skill;
   - the skill's routing rows, which a routed run adds to the project's root `AGENTS.md`;
   - the cases, in `claude plugin eval`'s format ([ADR 17](0017-repository-data-and-state.md) item 5), each with a `cech.toml` for the keys that format rejects.
3. **Each plugin has at least** one content case, one control case, three trigger cases that should load the skill (one avoiding the description's words), and three that should not (one a near miss: close to the skill's subject but outside it).
4. **Graders are checked offline.**
   Each content and control case has a deterministic grader of the reply or the files, which works on both harnesses.
   A grader may run a script on the files a run wrote, for formats such as `.docx` that a text grader cannot read.
   For each harness, each case keeps a recorded run that must pass and one that must fail, holding no local paths, session ids or account data.
5. **The eval gate** is `cech eval --check`, a merge gate ([ADR 5](0005-gates-and-tool-pins.md) item 5) that runs no model and checks items 2 to 4.
6. **The runner.**
   `cech eval` drives both harnesses itself ([ADR 8](0008-a-marketplace-you-extend-not-fork.md) item 4), as [ADR 9](0009-native-harness-features-first.md) item 1 allows where the native tool falls short.
   It installs the plugin natively in its arm only, out of the other arm's reach.
   An error makes a run "no result", not a fail.

## Consequences

- Each plugin needs a fixture project, a routing row, eight cases and their recordings.
- CECH maintains a runner that follows Codex's event output.
- A skill route meant for both harnesses belongs in the root `AGENTS.md`.

## Options considered

- **`claude plugin eval` as the Claude Code runner:** no routing, and results unlike Codex's.
- **An existing runner** (Harbor 0.25.0, BenchFlow 0.7.11, promptfoo 0.125.1, inspect_swe 0.2.72, skillgrade 0.5.0, skill-up 0.14.0; checked on 2026-10-29): none installs a plugin on both harnesses and detects a Codex load.
- **Cases inside each plugin:** graders and recordings would ship to users.
- **Text graders only:** a `regex` on a file reads a zipped format such as `.docx` as raw bytes, and an LLM grader cannot be checked offline.
