# 8. Categories and names

- Status: accepted
- Date: 2026-10-16
- Deciders: Michał Bożek

## Context

CECH will hold many single-skill plugins ([ADR 7](0007-one-plugin-one-skill.md)), so people need to browse them by kind.
Categories and names are decided together because a category in the name would rename a plugin whenever it moves.

Names clash and are hard to change (checked on 2026-10-16, versions as in [ADR 6](0006-claude-code-and-codex-from-one-source.md)):

- Neither tool puts the marketplace in a skill's name.
  Given two enabled plugins named `issues`, Claude Code loads one without warning and Codex lists both under one name.
- Projects commit enablement keys such as `issues@cech`, so a rename breaks them.

## Decision

1. CECH's categories are a closed list.
   Adding one is a dated amendment ([ADR 1](0001-record-architecture-decisions.md)) directly under the table; removing or merging one needs a new ADR.
   CI checks that `data/categories.toml` matches the table plus those amendments.

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
3. CECH plugin names are bare: `prose`, `docker`.
   They use `a-z`, `0-9` and `-`, start with a letter, do not end with `-`, contain no `--`, and have at most 64 characters.
   Such names are valid in both tools and in Agent Skills.
4. Each extension prefixes its plugins with its namespace (`acme-tracker`).
   The namespace is the extension's marketplace name.
   When chosen, it must not be a prefix of a CECH plugin name that ends before a `-` (CECH's `git-hooks` rules out `git`), so its plugin names cannot equal a CECH name.
   CECH keeps no registry of namespaces, so later clashes fall to item 7.
5. Marketplace names `cech` and `cech-*` are reserved for CECH.
   Each pinned release gets its own name, so several can be installed side by side (a later ADR on pinning).
   Item 7 still forbids enabling one plugin from two of them at once.
6. Extensions may add categories (e.g. `brand`) to their own catalogue.
7. `cech check` fails when two enabled plugins share a name.

## Consequences

- Short names and invocations (`prose:prose` in Claude Code, `$prose` in Codex).
- Moving a plugin to another category does not rename it.

## Options considered

- **Category prefix** (`writing-prose`): longer, renames on a move, and does not stop clashes with extensions.
- **`cech-` prefix on core plugins:** longer; extension prefixes make clashes rare, and `cech check` catches the rest.
- **Fewer, broader categories:** ends in a catch-all bucket.
- **Open categories:** the list fragments.
