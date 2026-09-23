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

## X post

276 of 280 characters (a URL counts as 23). The first line carries the finding;
the claim stays at "funded by the same address", never "the same person".
Attach `out/demo.mp4`; the GitHub link has to be in the post itself.

> 67 of the top 172 smart money wallets on @nansen_ai were funded by just 13 addresses.
>
> So "5 smart money wallets are buying" may be fewer independent decisions than it looks.
>
> I built a tool that tries to break a smart-money signal before you copy it.
>
> https://github.com/watarus/smart-money-falsifier

## Repository

TODO — push and paste the GitHub URL.

## Demo video

TODO — paste the X post URL. Must tag @nansen_ai.

## API call count

TODO — `wc -l out/calls.jsonl` after the full run, and confirm it against the
usage counter in the Nansen dashboard before submitting. Report the dashboard
number if the two disagree.
