# Upstream entries in a skill's NOTICE

The format of a forked skill's `NOTICE`, the change comments in files from `Apache-2.0` upstreams, the `cech validate` checks of both, and the scheduled upstream job, for [ADR 23](../../../docs/adr/0023-forking-upstream-skills.md) items 3 to 6.
Nothing here is built yet; the change that builds `cech validate` or the job takes these rules.

## Example

```text
cech-prose
Copyright (c) 2026 Michał Bożek and CECH contributors

This skill is licensed under the MIT License; see LICENSE. It adapts the
upstream text listed below, which keeps its own licence.

Upstream: https://github.com/acme/writing-skill
Commit: 3f2c9e1b7a4d5c6e8f9012a3b4c5d6e7f8091a2b
Path: skills/writing/SKILL.md
Path: skills/writing/patterns.md
Licence: MIT

Changed by CECH contributors.

Copyright (c) 2026 Acme Contributors

Permission is hereby granted, free of charge, ...
```

A changed file from an `Apache-2.0` upstream, here `SKILL.md`:

```markdown
---
name: cech-prose
description: ...
---
<!-- Changed by CECH contributors from https://github.com/acme/writing-skill -->
```

## Format

1. A skill folder's `NOTICE` is UTF-8 with LF line ends.
   Text before the first entry is free, but no line in it starts with `Upstream:`.
2. An entry starts at a line `Upstream: <url>`.
   The URL is `https://`, with a host and a path, and no query, fragment or credentials.
3. An entry's *header* is the lines from `Upstream:` to the first blank line.
   After `Upstream:` it holds, in this order and with nothing between them:
   - one `Commit: <hash>`, 40 or 64 lowercase hexadecimal characters;
   - one or more `Path: <path>`, relative to upstream's root, with `/` separators, no leading `/`, no `.` or `..` part, and no path repeated in the entry;
   - one `Licence: <id>`, an SPDX id from ADR 23 item 2, spelled exactly.
     When upstream offers a choice, such as `MIT OR Apache-2.0`, the field holds the one id the text is taken under, and the notice holds that licence.
4. After the header's blank line comes one line: `Changed by CECH contributors.`, or `Copied unchanged.` when CECH changed none of the text.
   Then come a blank line and the notice.
   The notice runs to the next `Upstream:` line or the end of the file, and has at least one line that is not blank.
5. Each key in a header is followed by one space and a value with no leading or trailing space.
   A header line with any other key, or a key out of order or repeated, is a finding.
6. Two entries in one `NOTICE` do not share an `Upstream:` URL; one entry lists every path taken from that upstream.

## Change comments

7. A file in a skill folder that holds changed text from an `Apache-2.0` upstream carries the line `Changed by CECH contributors from <url>`.
   `<url>` is the `Upstream:` URL of an entry in that folder's `NOTICE`.
   The line sits inside the file type's comment syntax:
   - in Markdown, as an HTML comment on the first line after the frontmatter, or on the first line where there is none;
   - in a script, as a comment after any `#!` line.
8. A file in a format with no comments, such as JSON, is taken from an `Apache-2.0` upstream only unchanged.
   Upstream's notices already inside a file stay as they are.

## `cech validate`

9. Every `NOTICE` in a skill folder has at least one entry.
   A skill folder holds a `NOTICE` only for third-party text ([ADR 22](../../../docs/adr/0022-two-install-paths.md) item 4), and ADR 23 item 1 makes any skill with such text a fork.
10. Each `Apache-2.0` entry marked `Changed by CECH contributors.` is named by at least one change comment in its folder, and each change comment names an `Apache-2.0` entry so marked in that folder's `NOTICE`.
11. `plugin.json` `license` starts with the marketplace's own licence (`MIT` in CECH).
    Each other licence of an entry in the skill's `NOTICE` follows once, in the order of ADR 23 item 2, joined by ` AND `.
    With no `NOTICE`, the field is the marketplace's own licence alone.
    The check covers only the terms after the first, since an extension chooses its own licence.
12. Each break of rules 1 to 6 and 9 to 11, and a change comment in the wrong form or place, is a finding that names the file and line; validation then exits 1 ([ADR 15](../../../docs/adr/0015-the-cech-cli.md) item 5).
13. It reads only the files in the tree, never the network.
    The contributor finds the files that hold changed `Apache-2.0` text (rule 7) and keeps files with no comment syntax unchanged (rule 8); review checks both.
14. The root `NOTICE` is not checked by these rules.

## The scheduled upstream job

15. A mise task, not part of `check`, reads every skill folder's `NOTICE` and, for each entry, fetches the pinned commit by hash and upstream's default branch, without file contents (`git fetch --filter=blob:none`), then:
    - reports when upstream cannot be reached after one retry, or the fetch of the pinned commit fails;
    - reports when any listed path differs between the pinned commit and the head of the default branch, naming the newest upstream commit that changed a listed path.
16. For tests, an environment variable that only the job reads may point an `Upstream:` URL at a local repository; `cech validate` never accepts one.
17. A scheduled workflow in `.github/workflows/` runs the task weekly, never on a pull request.
18. It keeps one open issue per skill and upstream, labelled `upstream` and found by the marker `<!-- cech-upstream: <skill> <url> -->` in its body.
    It updates that issue rather than opening another, an unreachable upstream included.
    If the issue was closed as not planned, the job stays silent for that skill and upstream until a listed path changes in a commit after the one that issue named.
19. An issue holds only values from the tree's `NOTICE` and commit hashes: no upstream commit messages or file text.
    Upstream values reach a `run:` step only through environment variables, quoted.
20. The workflow has `contents: read`, widened by `issues: write` only for a separate job that writes the issue ([ADR 4](../../../docs/adr/0004-gates-and-ci.md) item 6).
