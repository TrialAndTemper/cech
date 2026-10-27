# 8. Categories and names

- Status: accepted
- Date: 2026-10-16
- Deciders: Michał Bożek

## Context

CECH will hold many single-skill plugins ([ADR 7](0007-one-plugin-one-skill.md)), so people need to browse them by kind.
Categories and names are decided together because a category in the name would rename a plugin whenever it moves.

Names clash and are hard to change (checked on 2026-10-16, versions as in [ADR 6](0006-claude-code-and-codex-from-one-source.md)):

- Neither harness puts the marketplace in a skill's name.
  Given two enabled plugins named `issues`, Claude Code loads one without warning and Codex lists both under one name.
- Projects commit enablement keys such as `cech-issues@cech-0-1-0`, so a rename breaks them.

Plugins also come from marketplaces that follow no CECH rule, and a skill installer can copy a skill without its plugin into a project or user skills directory, where only its name identifies it (checked on 2026-10-16 with `npx skills` and `gh skill`, as in [ADR 3](0003-a-marketplace-you-extend-not-fork.md)).

## Decision

1. CECH's categories are a closed list.
   Adding one refines this ADR, with an amendment note directly under the table; removing or merging one needs a superseding ADR.
   Until the first plugin merges, either is an edit in place instead ([ADR 1](0001-record-architecture-decisions.md) item 5).
   CI checks that `data/categories.toml` matches the table, together with any amendment notes under it.

   | Category | Holds |
   |---|---|
   | `practice` | Tool-neutral engineering rules (testing, coding, config, logging, dependencies, containers, infrastructure as code) |
   | `writing` | Prose, docs, decision records, changelogs, commit messages |
   | `agent` | How agents work in a repository (instruction files, task records, estimates, change specs) |
   | `workflow` | Multi-step flows that move a change through stages |
   | `lang` | Languages and runtimes |
   | `framework` | Libraries and frameworks |
   | `tool` | One command-line tool each |
   | `platform` | Hosted services: forges, trackers, clouds, observability |
   | `meta` | Authoring and testing CECH skills |

2. The category is the plugin's folder (`plugins/<category>/<name>/`).
   The build generates the marketplace catalogue, including its `category` field, from it.
   It is not part of the name.
3. CECH plugin names start with `cech-`: `cech-prose`, `cech-docker`.
   The skill inside has the same name, with no alias.
   Names use `a-z`, `0-9` and `-`, start with a letter, do not end with `-`, contain no `--`, and have at most 64 characters.
   Such names are valid in both harnesses and in Agent Skills.
4. Each extension prefixes its plugin names with `<namespace>-` (`acme-tracker`).
   The namespace is the name the extension's catalogue carries outside a release tag; each release adds its version to it, as CECH's do (item 5).
   It is not `cech` and does not start with `cech-` (item 5), so its plugin names cannot equal a CECH name.
   CECH keeps no registry of namespaces, so clashes between extensions fall to item 7.
5. CECH's namespace is `cech`, and marketplace names `cech` and `cech-*` are reserved for CECH.
   Each pinned release gets its own name, carrying its version, so several can be installed side by side (a later ADR on pinning).
   Item 7 still forbids enabling one plugin from two of them at once.
6. Extensions may add categories (e.g. `brand`) to their own catalogue, listed in their `data/marketplace.toml` (a later ADR on extensions).
7. `cech check` fails when two enabled plugins share a name.

## Consequences

- No CECH name can equal a name without the `cech-` prefix, whether it comes from an extension, another marketplace or a skill installed without its plugin; a third party that copies the `cech-` prefix still clashes, and item 7 catches that only between enabled plugins.
- Longer names and invocations (`cech-prose:cech-prose` in Claude Code, `$cech-prose` in Codex), and keys repeat the namespace (`cech-prose@cech-0-1-0`).
- Moving a plugin to another category does not rename it.

## Options considered

- **Category prefix** (`writing-prose`): longer, renames on a move, and does not stop clashes with extensions.
- **Bare names** (`prose`): shorter, but extension prefixes stop only clashes with extensions, not with other marketplaces or skills installed without their plugin.
- **Bare names with a prefixed alias for skills installed without their plugin:** two names per skill to keep in step.
- **Fewer, broader categories:** ends in a catch-all bucket.
- **Open categories:** the list fragments.
