# 28. Telemetry

- Status: accepted
- Date: 2026-11-26
- Deciders: Michał Bożek

## Context

Telemetry from real use could show how skills load, cost and fail, but a raw export also carries prompts, paths, other plugins, and account and session ids.
Read on 2026-11-18 with Claude Code 2.1.324, and on 2026-11-26 with Codex CLI 0.171.0:

- neither harness sends its OpenTelemetry data to a marketplace: the user or the organisation sets where it goes;
- neither harness has a hook event for a skill load, and a plugin hook runs on the user's machine with the user's rights.

## Decision

1. **CECH sends no telemetry.**
   No CECH plugin, skill, hook or command sends data about how its users use it.
   CECH measures its skills only in the eval runs it starts itself ([ADR 27](0027-the-release-check.md)).
2. **Organisations may share summaries of their own telemetry.**
   `cech` makes a summary of how CECH's skills were used from an organisation's own telemetry export, and sends nothing.
   It gives counts and medians per skill and version over one period.
   It holds no ids, prompts, paths, hosts, model deployments or other plugins, and no figure from so few people that it could identify one.
   The organisation reads it and, if it chooses, submits it by hand; CECH keeps summaries in its public issue tracker, under the submitter's name, never in its tree.
3. **The summary's format is on the CLI's surface** ([ADR 24](0024-what-a-version-change-means.md) item 3), so a change to it follows the CLI's version rules.

## Consequences

- CECH learns how its skills are used only from issue reports, its own eval runs and the summaries organisations choose to share; a person may turn an issue report into eval cases.
- Summaries show use, not whether a skill helped; evals measure that.
- An organisation that wants to measure CECH's skills turns on its harness's telemetry, sent to an endpoint it chooses.

## Options considered

- **Telemetry sent to CECH,** by a plugin hook or an opt-in stream: a hook would send data without the user's consent, and a stream needs an endpoint, a privacy review and consent handling, and would carry the whole session.
- **Organisations sharing their telemetry exports:** an export carries prompts, paths, other plugins, and account and session ids; a summary carries only counts and medians.
