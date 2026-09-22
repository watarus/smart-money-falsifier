# Submission draft

The entry form asks for exactly three things: **email address, the X post URL,
and the GitHub link**. Everything else below is material for the X post and the
README, not form fields.

Checklist from the official rules
(academy.nansen.ai/en/help/articles/3540155):

- [ ] 1,000+ API calls logged **between Sep 14 and Sep 27** — calls before
      Sep 14 do not count
- [ ] GitHub repo is **public** (private repos are disqualified)
- [ ] X post tags `@nansen_ai` and includes the GitHub link
- [ ] 30–60s screen recording showing the build running with live Nansen data,
      followable **without audio**
- [ ] One submission per account
- [ ] Submitted before Sep 27, 23:59 UTC

Judging is four equally weighted criteria: Data Integration, Functionality,
Creativity & Originality, and Documentation & Submission.

## Project name

Smart Money Copy-Trade Scorecard

## One-liner

Ranks Nansen Smart Money wallets by how worth-following they look right now, and
shows what they are quietly accumulating.

## Description (~150 words)

Nansen's Smart Money feed tells you what top wallets are buying. It doesn't tell
you which of those wallets is actually worth copying today, or whether the token
they just bought is being accumulated or distributed.

This CLI closes that gap. A single `smart-money/dex-trades` call across every
supported chain yields ~172 labelled wallets and ~330 tokens. Each wallet is
enriched with its realised PnL, win rate and related-wallet cluster; each token
with smart-trader, whale and exchange net flow plus market structure. The result
is a ranked scorecard: which wallets have earned conviction, and which of their
current positions are backed by flow rather than hype.

It is built to respect the API's economics. Every response is cached to disk, the
client refuses calls that would breach a credit floor, and each request is logged
with its Nansen request ID so a run can be reconciled call-for-call.

## Which Nansen API endpoints were used

- `smart-money/dex-trades` — seed, one call across all chains
- `profiler/address/pnl-summary` — per-wallet realised PnL, win rate, trade count
- `profiler/address/related-wallets` — wallet clustering and funder relations
- `tgm/flow-intelligence` — smart trader / whale / exchange net flow per token
- `tgm/token-information` — market cap, liquidity, holders, token age

## X post draft

The post carries the video, so the first line has to work as a scroll-stopper
on its own. Keep the finding concrete — the numbers are the hook, not adjectives.

> Seven "independent" smart money wallets bought the same token.
>
> They were one person.
>
> I built a tool on @nansen_ai that tries to *falsify* smart-money signals
> instead of copying them. On a 50-wallet sample, 16 of the top 50 smart money
> wallets traced back to just two funders.
>
> It walks the funder graph to collapse apparent independence, and flags tokens
> where smart traders are accumulating while the token flows into exchanges —
> i.e. you're buying someone's exit.
>
> Go CLI, every API response cached, every call receipted with its Nansen
> request id.
>
> github.com/<user>/smart-money-falsifier

Attach the 30–60s recording. Do not put the GitHub link only in a reply — the
rules ask for it in the post itself.

## Repository

TODO — push and paste the GitHub URL.

## Demo video

TODO — paste the X post URL. Must tag @nansen_ai.

## API call count

TODO — `wc -l out/calls.jsonl` after the full run, and confirm it against the
usage counter in the Nansen dashboard before submitting. Report the dashboard
number if the two disagree.
