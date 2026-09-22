# 30–60 second demo script

The rules require a 30–60 second screen recording showing the build running with
live Nansen data, **followable without audio**. So every line below has to land
on screen — as a typed command, as visible output, or as a burned-in caption.
Write the captions first and treat narration as optional.

Record after a run, so the cache is warm and every command returns instantly. A
demo that pauses on network latency reads as a broken demo.

Terminal at a large font, dark theme, window cropped to the text.

## The one thing this video has to land

Not "here is a dashboard". This:

> These "independent" smart-money buyers were funded from the same address.

Everything else is supporting evidence. If a viewer with the sound off comes
away with that sentence, the video worked.

## Shot list

**0:00–0:08 — the claim everyone else makes**

Open on the seed data: real Nansen labels scrolling past — `STONK Whale`,
`HL Perps Swing Trader`, `sunknight.eth`. Caption:

> "Nansen shows you what smart money is buying. Every tool built on it says:
> copy them."

**0:08–0:18 — the question**

```
./scorecard --offline
```

Caption as the token table renders:

> "This one asks whether the signal is real."

Let the verdict column land. `CONCENTRATED`, `DISTRIBUTING`, `CONFIRMED` in
colour against real token symbols.

**0:18–0:35 — the collapse (the money shot)**

Expand one `CONCENTRATED` row. Show `N → M`: several distinct smart-money
buyers resolving to fewer funding sources, with the shared funder named.
Prefer a small-cap token over USDC/ETH/SOL — "smart money bought USDC" was
never a signal, so collapsing it proves little.

> "Seven 'independent' buyers. One first funder."

Hold this. It is the whole video. If the run surfaces the `realkingof.sol`
cluster — 10 wallets, one funder — use that row.

**0:35–0:48 — the other lie**

Scroll to a `DISTRIBUTING` row and let the two signed numbers sit side by side:
smart-trader net flow positive, exchange net flow positive.

> "Smart traders accumulating. The same token flowing into exchanges. You are
> buying someone's exit."

**0:48–0:60 — the receipts**

```
wc -l out/calls.jsonl
tail -1 out/calls.jsonl
```

> "Every call logged with its Nansen request ID and credit cost."

## Notes

- Captions carry the video. Anything you would have said out loud needs to be on
  screen for the same beat.
- Show **live Nansen data** — real addresses, real labels, real flows.
  Cached-but-real counts; placeholder data does not.
- Do not show `.env` or any terminal where the key could appear. Check the
  scrollback before recording.
- If a number on screen looks implausible, show it anyway and say so. Judges
  notice hidden rows more than ugly ones.
- Documentation is weighted equally with the other three criteria, so link the
  README from the X post, not just the repo root.
- Tag `@nansen_ai` and include the GitHub link in the post.
