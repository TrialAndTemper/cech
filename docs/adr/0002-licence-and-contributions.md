# 2. Licence and contributions

- Status: accepted
- Date: 2026-10-16
- Deciders: Michał Bożek

## Context

Users, including companies with private extensions, must know they may use, change and share CECH.
Contributors must know what they agree to.
Some skills will build on other people's ideas or adapt text from other open-source projects.

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
4. **Third-party text.**
   Rewrite it where you can.
   Copied or adapted text needs a compatible licence, its notice in the root `NOTICE` file, and a source note in the skill naming the upstream project and the commit it came from.

## Consequences

- Companies can build private extensions on CECH with few limits.
- Contributors add `-s` to every commit.
- A skill may wait for its idea's author to answer.

## Options considered

- **Apache-2.0:** patent grant, but longer terms; the grant matters little for skill text.
- **MIT for code, CC BY 4.0 for text:** two licences in one plugin hinder reuse.
- **CLA:** allows relicensing, but deters contributors and needs a bot.
