# 3. Licence and contributions

- Status: accepted
- Date: 2026-10-16
- Deciders: Michał Bożek

## Context

Users, including companies with private extensions, must know they may use, change and share CECH.
Contributors must know what they agree to.
Some skills will build on other people's ideas or adapt text from other open-source projects, and the CLI builds in third-party Go modules.
A licence fits only if CECH can meet its terms inside what it ships, a skill folder that an installer may copy on its own, or the CLI binary, and if it does not bind the organisations that extend CECH.
A skill installer, such as `npx skills`, can copy one skill folder out of the repository and leave the repository's root files behind.

## Decision

1. **MIT** covers everything in the repository unless a file or folder says otherwise.
   Its missing patent grant is accepted for skill text and revisited before the first `cech` CLI release.
2. **DCO, no CLA.**
   Every commit carries `Signed-off-by:` (`git commit -s`), stating the author may submit it under the licence that covers the file ([developercertificate.org](https://developercertificate.org/)).
   CI rejects commits without it.
3. **Consent before shipping.**
   A skill based on a specific person's unpublished work ships only after they agree; with no answer, the idea stays out.
   Published ideas are credited by citing the publication.
   Naming a person beyond that needs their consent, and they choose the name and email shown, if any.
4. **Third-party text and code.**
   Rewrite text in your own words where you can; a skill written from an idea alone needs only the credit of item 3.
   Text copied from another project, or adapted from it by translating, closely paraphrasing or reordering it, needs a licence that is permissive, not copyleft, and whose terms can be met inside the skill folder or the binary that ships it, and so does a Go module the CLI builds in.
   The SPDX ids that meet this are listed where the check that enforces it reads them; adding one is a reviewed change to that list.
   Copied or adapted text also needs its notice, and a source note in the skill naming the upstream project and the commit it came from.
   The notice for text in a skill goes in a `NOTICE` file in the skill folder, so it travels with the folder; any other notice goes in the root `NOTICE` file.

## Consequences

- Companies can build private extensions on CECH, and MIT asks only that they keep its notice.
- Contributors add `-s` to every commit.
- The people whose ideas a skill builds on are credited, and a skill based on unpublished work waits for its author's consent.

## Options considered

- **Apache-2.0:** patent grant, but longer terms; the grant matters little for skill text.
- **MIT for code, CC BY 4.0 for text:** two licences in one plugin hinder reuse.
- **CLA:** allows relicensing, but deters contributors and needs a bot.
- **Copyleft upstreams:** their terms can bind organisations that extend CECH.
- **CC BY 4.0 for third-party text:** the [Creative Commons FAQ](https://creativecommons.org/faq/#can-i-apply-a-creative-commons-license-to-software), read on 2026-10-29, advises against CC licences for software, such as a skill's scripts.
- **The accepted licences listed in this ADR:** each added licence would need an amendment note, and the lists for skills and Go modules would sit apart from the checks that read them.
