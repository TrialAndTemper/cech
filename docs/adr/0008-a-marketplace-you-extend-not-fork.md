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
Telemetry from real use could show how skills load, cost and fail, but a raw export also carries what organisations do, their other plugins, and account and session IDs.
Neither harness sends its OpenTelemetry data to a marketplace: the user or the organisation sets where it goes (read on 2026-11-18, Claude Code 2.1.324 docs, Codex CLI docs).

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
   It validates a marketplace on top of the harnesses' own validation, runs evals on both harnesses, since Codex has no eval command and `claude plugin eval` loads no project instructions (checked on 2026-10-29, Claude Code 2.1.312, Codex CLI 0.165.0), and checks a project across its enabled marketplaces.
5. **CECH does not build** a package manager, a multi-harness installer, an eval runner where a harness's own does the job, an `AGENTS.md` sync tool, or telemetry sent from its users to CECH: no CECH plugin, skill, hook or command sends data about how its users use it.
   CECH keeps to formats that the harnesses and existing tools read.
6. **CECH versions its plugins and roles by marketplace release, and its layout rules by CLI release.**
   Each extension release names the one CECH release it builds on.
7. **Organisations may share summaries of their own telemetry.**
   `cech` makes a summary of CECH's skills from an organisation's own export and sends nothing: counts and medians per skill and version over one period, with no IDs, prompts, paths, hosts, model deployments or other plugins, and no figure from so few people that it could identify one.
   The organisation reads it and, if it chooses, submits it by hand; CECH keeps summaries in its public issue tracker, under the submitter's name, never in its tree.

## Consequences

- Fixes reach extension users through a normal upgrade.
- Each CECH release needs a release of every extension that builds on CECH plugins before projects using that extension can take it.
- CECH learns how its skills are used only from issue reports (which a person may turn into eval cases), its own eval runs and the summaries organisations choose to share.
- Summaries come only from organisations that choose to share, and show use, not whether a skill helped; evals measure that.
- The summary is a `cech` output that organisations rely on, so its format changes only with a CLI release.

## Options considered

- **A template to fork:** forks drift.
- **One marketplace that takes company content:** breaks neutrality.
- **Loose skill folders, no marketplace:** no versions, no per-project enablement, no shared tooling.
- **Skills installed by skill installers as the only path** (`npx skills`, `gh skill`; checked on 2026-10-16): they record and update skills, but put a copy of each skill in every repository or user directory, without plugin parts (reviewer agents, hooks, MCP servers) or dependencies.
  Item 2 still allows them for skills that need no plugin parts, dependencies or role binding.
- **A cross-harness package manager** (`dotagents`, `apm`; checked on 2026-10-16): they pin per repository, but by copying files into each one, so every repository holds its own copy.
  They can still install CECH plugins, since the format is shared.
- **Telemetry sent to CECH,** by a plugin hook or an opt-in stream: a hook would send data without the user's consent, and a stream needs an endpoint, a privacy review and consent handling, and would carry the whole session.
- **Organisations sharing their telemetry exports:** each export is the organisation's private data, which CECH keeps out of its tree ([ADR 4](0004-vendor-neutrality.md)); a summary carries only counts and medians.
