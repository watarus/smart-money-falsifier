# Is that smart-money signal real? — design

Nansen's Smart Money feed tells you what top wallets are buying. Every other
tool built on it ranks those wallets and copies them. This one does the
opposite: it tries to **falsify** the signal before you act on it.

Two ways a "seven smart-money wallets are buying this" headline lies:

1. **The buyers are not independent.** Walk each wallet's funder graph and the
   seven collapse into one actor who split size across seven addresses.
2. **The accumulation is someone else's exit.** Smart traders are net buying
   while the token is simultaneously flowing *into* exchanges.

Both are only visible because Nansen labels addresses and exposes funder
relations. No block explorer gives you this.

## What the real data says

Measured over the full seed (`out/calls.jsonl`, 1019 calls, 0 failures):

- **67 of the 172 wallets trace back to just 13 funders**: `realkingof.sol`
  funded 20, an address Nansen labels `Token Millionaire` funded 14,
  `kiing.sol` funded 7, and ten more funded between 2 and 5 each. None is an
  exchange, so this is not the usual "everyone withdrew from Binance" artefact.
- `exchange_net_flow_usd` is populated for 198 of 330 tokens (60%).
- The exit signal fires on 32 of 330 tokens (10%) — rare enough to be worth
  flagging, common enough to be worth computing.
- Of the 56 tokens with at least 3 buyers: 9 `CONCENTRATED`, 15 `CONFIRMED`,
  32 `DISTRIBUTING`, 0 `UNVERIFIED` once funder coverage is complete.

The pitch is not hypothetical; these numbers are the demo.

## Relations that mean something

`relation` on `profiler/address/related-wallets` is a free-form string, not an
enum. Observed values and how to treat them:

| relation | observed | use |
| --- | ---: | --- |
| `Deployed Program` | 187 | ignore — contract artefact |
| `First Funder` | 182 | **the clustering edge** |
| `Deployed Contract` | 23 | ignore |
| `Deployed via` | 18 | ignore |
| `Deployed by` | 7 | ignore |
| `Created by` | 7 | ignore |

Only `First Funder` links a wallet to a funding actor. The deployment relations
attach a wallet to contracts it touched and would merge unrelated wallets into
one giant bogus cluster. Unknown future relations must be ignored by default,
never treated as a funding edge.

## Cluster collapse

For each token, take the set of seed wallets that **bought** it.

1. Map each wallet to its `First Funder` addresses.
2. Union two wallets when they share a funder address, or when one wallet's
   funder is itself another buyer of the same token.
3. `N` = distinct buying wallets. `M` = distinct clusters after the union.

`N → M` is the headline number. `N = 7, M = 1` means one actor wearing seven
hats. `N = 7, M = 7` means seven independent decisions.

A wallet with no funder data is its own cluster — absent data must never
manufacture a collapse.

**Absent data must not manufacture independence either.** That rule cuts both
ways, and the first implementation only honoured one direction. Run the tool
over the full seed with only part of the funder data cached and every
unenriched wallet becomes its own cluster, so `M` climbs toward `N` and the
token is declared `CONFIRMED` — "no two buyers share a funder" — when the
question was never asked. Observed live: 🌱KEK went from an honest `—` at 50
wallets to `3 → 3 DISTRIBUTING` over the full seed purely because two of its
buyers had no `related-wallets` response cached.

So every token carries **funder coverage**: how many of its `N` buyers actually
have a `related-wallets` result. Coverage below 100% means the independence
question is open:

- Report `N → M` together with coverage, e.g. `4 → 2 (3/4 checked)`.
- `CONFIRMED` requires full coverage. With any buyer unchecked, the verdict is
  `UNVERIFIED`, never `CONFIRMED` — a collapse found on partial data is still
  real (finding a shared funder proves dependence), but *failing* to find one
  across unchecked wallets proves nothing.
- `THIN` and `CONCENTRATED` remain valid under partial coverage, since they rest
  on funder matches that were positively observed.

## Exit signal

`*_net_flow_usd` is net flow **for addresses carrying that label**, so a
positive `exchange_net_flow_usd` means value moving *into* exchange addresses —
deposits, i.e. sell-side pressure. Negative means withdrawal.

Distribution fires when `smart_trader_net_flow_usd > 0` **and**
`exchange_net_flow_usd > 0`: the cohort is accumulating into a token that is
being deposited to exchanges at the same time.

Always render both raw signed numbers next to the verdict. A reader must be
able to disagree with the label by looking at the inputs.

## Verdict

The first version of this table had a hole: `N >= 3, M == 2` satisfied neither
`THIN` (`M == 1`) nor `CONFIRMED` (`M >= 3`), so ETH (4 → 2), USDG (3 → 2) and
ZCAT (3 → 2) all fell through to `WEAK` despite a real, measured collapse. The
rules below are total — every `(N, M)` lands somewhere, and the independence
verdict is decided before the distribution overlay.

Independence, on tokens with `N >= 3` buyers:

| verdict | condition | meaning |
| --- | --- | --- |
| `THIN` | `M == 1` | one actor wearing N hats |
| `CONCENTRATED` | `1 < M < N` | some of the apparent independence is illusory |
| `CONFIRMED` | `M == N` | no two buyers share a funder |

Then the distribution overlay: if the exit signal fires, `CONFIRMED` becomes
`DISTRIBUTING`, and `THIN`/`CONCENTRATED` become `BOTH`.

`M == N` is a stronger and more honest definition of confirmed than "at least
three clusters": it says exactly what was checked — no shared funder was found
between any two buyers.

Tokens with `N < 3` buyers carry no independence signal at all; see Output for
how they are presented.

Verdicts are categorical. They are not a 0..1 score, because the previous
version's continuous token score saturated — all 20 rendered tokens scored
exactly `1.000`, which is how a normalisation bug hides in plain sight.

Any continuous score that survives must be rank-based within the cohort, and a
test must assert that distinct inputs produce distinct outputs.

## Hard constraint: credits

Credits are consumed per call and priced per endpoint. Balance was 1143 after
the full run. The buildathon wants 1,000+ calls logged between Sep 14 and
Sep 27, but **calls are not the goal** — fetch what the product needs while
developing and the count accumulates on its own. Do not add stages to inflate it.

| endpoint | credits | role |
| --- | ---: | --- |
| `smart-money/dex-trades` | 5 | seed, called once with `per_page: 1000` |
| `profiler/address/pnl-summary` | 1 | who is in the cluster |
| `profiler/address/related-wallets` | 1 | the funder edges |
| `tgm/flow-intelligence` | 1 | the exit signal |
| `tgm/token-information` | 1 | name, symbol, liquidity |

Rules that stay:

- Every response is cached to `data/cache/<sha256(path+body)>.json` before
  anything parses it. Re-runs, tests, and the demo cost zero credits.
- The client refuses a call that would drop the balance below `--credit-floor`
  (default 100) and refuses endpoints above 1 credit without
  `--allow-expensive`. Fail closed.
- `out/calls.jsonl` records every call with its `request_id`, so a run
  reconciles against Nansen's own counter.

## API facts (verified live)

- Base `https://api.nansen.ai/api/v1/`, **every endpoint is POST** with a JSON body.
- Auth header is `apikey: <key>`, not `Authorization`. The binary reads the
  `NANSEN_API_KEY` env var; it does not parse `.env`.
- Free tier 15 req/s, 300 req/min; `429` carries `Retry-After`.
- Responses carry `X-Nansen-Credits-Cost`/`-Used`/`-Remaining` and `X-Request-Id`.
  A request rejected before pricing deducts nothing.
- `pagination.per_page` maxes at 1000.
- `chains: ["all"]` works on the seed and `chain: "all"` on `pnl-summary`, but
  `related-wallets` **rejects** `chain: "all"` (422 `invalid_field_value`), so it
  is called once per (wallet, chain) pair seen in the seed.
- The native-token pseudo-address `0xeeee…eeee` is accepted by TGM endpoints and
  resolves to the chain's native asset; it is not a dead call.

Request bodies:

```jsonc
{"chains": ["all"], "pagination": {"page": 1, "per_page": 1000}}         // seed
{"address": "0x..", "chain": "all", "date": {"from": "..", "to": ".."}}  // pnl-summary
{"address": "0x..", "chain": "ethereum"}                                 // related-wallets
{"chain": "ethereum", "token_address": "0x..", "timeframe": "1d"}        // flow-intelligence
{"chain": "ethereum", "token_address": "0x..", "timeframe": "1d"}        // token-information
```

## Output

The token table is the product; the wallet table is supporting evidence.

**The finding has to outrank the table.** The first rendering buried the whole
point: 75 of 84 token rows were `WEAK` single-buyer noise, and the funder census
— the one number worth remembering, *67 of 172 top smart-money wallets trace to
13 people* — sat in a small box above a wall of grey.

So:

- Lead with the census as a headline figure, stated as a sentence a viewer can
  read in one beat, naming the funders and their wallet counts.
- Rank the token table by how much it actually says: `BOTH`, then `THIN`, then
  `CONCENTRATED`, then `DISTRIBUTING`, then `CONFIRMED`.
- Tokens with `N < 3` buyers have no *independence* signal, so they carry no
  independence verdict. Collapse them behind a single disclosure line
  (`N tokens had fewer than 3 smart-money buyers — no independence signal`) so
  nothing is hidden, but nothing meaningless is promoted either.

**The buyer floor gates the independence verdict only — never the row.** The
exit signal needs no minimum buyer count: one smart-money wallet accumulating a
token that is simultaneously flowing into exchanges is exactly as damning as
five. A first pass at this filter dropped every `WEAK` row and took all seven
`DISTRIBUTING` tokens with it, deleting half the product from the report. Any
token whose exit signal fires belongs in the main table regardless of `N`, with
its independence column reading `—` rather than a verdict.

Per token, in this order: **symbol and name** (not a bare address — the previous
version rendered `solana:So111…112` with no indication it was wrapped SOL),
verdict, `N → M`, smart-trader net flow, exchange net flow, liquidity.

Clicking or expanding a token shows the buying wallets grouped by cluster, with
each wallet's Nansen label, realised PnL, and win rate, and the shared funder
named where one exists.

- Terminal table, colour-coded by verdict.
- `out/report.html`, self-contained, dark theme, suitable for a 30–60s screen
  recording that must be followable without audio.
- `out/calls.jsonl`, the call evidence.
