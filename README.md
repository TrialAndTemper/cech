# CECH

[![CI](https://github.com/TrialAndTemper/cech/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/TrialAndTemper/cech/actions/workflows/ci.yml?query=branch%3Amain)

CECH is a free, open-source marketplace of agent skills for Claude Code and Codex.
Each plugin holds one skill, written once and loaded by both tools.
Organisations extend CECH with marketplaces in their own repositories instead of forking it, and each project turns on the plugins it wants from CECH and the extensions it uses.
A small `cech` command-line tool will connect a project to CECH and its extensions and check the setup.

## Status

Early work: the repository holds the design decisions, the gates that check every change, and the start of the `cech` tool, which so far only prints its version.
It has no skills yet, so there is nothing to install.

## Learn more

- [Architecture decision records](docs/adr/README.md): the contracts CECH follows, and why.
- [Contributing](CONTRIBUTING.md): set-up, gates, commit rules and issue labels.
- [Discussions](https://github.com/TrialAndTemper/cech/discussions): questions and ideas.
- [Code of conduct](CODE_OF_CONDUCT.md)
- [Security policy](SECURITY.md)

## Licence

CECH is licensed under the [MIT License](LICENSE).
[NOTICE](NOTICE) lists the third-party material in the repository and the licence of each.
