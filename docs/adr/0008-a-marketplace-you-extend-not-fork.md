# 8. A marketplace you extend, not fork

- Status: accepted
- Date: 2026-10-16
- Deciders: Michał Bożek

## Context

The same agent guidance (commits, `AGENTS.md`, testing, review) is useful in many repositories.
Agent skills package it, and Claude Code and Codex both install skills from plugin marketplaces.
A shared set helps only if each organisation can add its own skills.
Forks of the set drift and miss fixes.
These ADRs call Claude Code and Codex harnesses: the programs that run an agent, as opposed to the tools an agent calls.

## Decision

1. **CECH is a marketplace of agent skills** for Claude Code and Codex.
2. **Organisations extend it in their own repositories.**
   An extension is another marketplace with the same layout.
   A project enables plugins from CECH and from any number of extensions.
   It commits only which plugins and versions it enables; the plugins are not copied into it.
   A team may instead copy single skills into a project or a home directory with a skill installer, without `cech`.
3. **Skills compose through roles.**
   A skill asks for "the issue tracker", and the project binds that role to a skill from any marketplace.
4. **Extensions reuse CECH's tooling** by installing the `cech` CLI, not by copying scripts.
   `cech validate` wraps `claude plugin validate` and adds CECH rules.
   `cech eval` wraps `claude plugin eval` and adds runs on Codex, which has no eval command of its own (checked on 2026-10-16, Codex CLI 0.162.1).
   `cech check` checks a project across its enabled marketplaces.
5. **CECH does not build** a package manager, a multi-harness installer, an eval runner where a harness has one, or an `AGENTS.md` sync tool.
   CECH keeps to formats that the harnesses and existing tools read.
6. **CECH versions its plugins and roles by marketplace release, and its layout rules by CLI release.**
   Each extension release names the one CECH release it builds on.

## Consequences

- Fixes reach extension users through a normal upgrade.
- Each CECH release needs a release of every extension that builds on CECH plugins before projects using that extension can take it.

## Options considered

- **A template to fork:** forks drift.
- **One marketplace that takes company content:** breaks neutrality.
- **Loose skill folders, no marketplace:** no versions, no per-project enablement, no shared tooling.
- **Skills installed by skill installers as the only path** (`npx skills`, `gh skill`; checked on 2026-10-16): they record and update skills, but put a copy of each skill in every repository or user directory, without plugin parts (reviewer agents, hooks, MCP servers) or dependencies.
  Item 2 still allows them for skills that need no plugin parts, dependencies or role binding.
- **A cross-harness package manager** (`dotagents`, `apm`; checked on 2026-10-16): they pin per repository, but by copying files into each one, so every repository holds its own copy.
  They can still install CECH plugins, since the format is shared.
