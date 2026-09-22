# Is that smart-money signal real?

Nansen's Smart Money feed tells you what top wallets are buying. Every other
tool built on it ranks those wallets and copies them. This one does the
opposite: it tries to **falsify** the signal before you act on it.

A "seven smart-money wallets are buying this" headline lies in two ways:

- **The buyers may not be independent.** Walk each wallet's funder graph and
  the seven may turn out to share one or two funding sources — which could
  be one trader splitting size, or a service that funds many wallets. Either
  way it is not seven independent decisions.
- **The accumulation is someone else's exit.** Smart traders are net buying
  while the token is simultaneously flowing *into* exchange addresses.

Both are only visible because Nansen labels addresses and exposes funder
relations. No block explorer gives you this.

Over the full Smart Money feed, **67 of the 172 top smart-money wallets were
funded by just 13 addresses** — `realkingof.sol` funded 20 of them, an address
Nansen labels `Token Millionaire` funded 14, `kiing.sol` funded 7. None is an
exchange, so this is not the usual "everyone withdrew from Binance" artefact.
The exit signal — smart traders buying while the token flows into exchanges —
fired on 32 of 330 tokens.

Of the 51 tokens it could judge, 9 came back `CONCENTRATED` and 7 `CONFIRMED`.
The tool clears signals as well as flagging them; that is what makes a flag
worth reading.

`CONFIRMED` means a signal survived both checks. It does not mean the token
is a good buy: the seven that pass are mostly days-old small caps, and the
tool knows nothing about their fundamentals.

It also doesn't mean you're early. X7 came out `CONFIRMED` correctly — five
independent buyers, money leaving exchanges — but had already run 30x past
the price smart money paid, and later gave most of it back. So every judged
token also shows its move since smart money bought: `peaked 30x, now 2.8x`.
Moves past 5x are highlighted. A signal that is real and late is still late.

Built with the [Nansen API](https://docs.nansen.ai/) for the Meridian Buildathon.

## Quick start

```sh
export NANSEN_API_KEY=...        # the CLI reads the env var, not .env
go build -o bin/scorecard ./cmd/scorecard

./bin/scorecard --dry-run        # show the call plan and credit cost, no HTTP
./bin/scorecard                  # run it (prompts before spending credits)
./bin/scorecard --offline        # re-render from cache, costs nothing
open out/report.html
```

## Why it is built the way it is

The Nansen API charges credits per call, priced per endpoint. Treating those
credits as a real budget shaped the design rather than being bolted on
afterwards — every one of the choices below exists because a careless loop over
a 5-credit endpoint is an expensive mistake you only make once.

**Only 1-credit endpoints fan out.** The 5-credit seed
(`smart-money/dex-trades`) is called exactly once, with `per_page: 1000`, across
all chains. Everything downstream — `profiler/address/pnl-summary`,
`profiler/address/related-wallets`, `tgm/flow-intelligence`,
`tgm/token-information`, `tgm/token-ohlcv` — costs 1 credit. `tgm/indicators` and
`profiler/address/counterparties` are useful but cost 5, so they are out.

**Nothing is ever fetched twice.** Every response is written to
`data/cache/<sha256(path+body)>.json` before anything parses it. Re-runs, tests,
the `--offline` mode, and the demo recording all cost zero credits. A run
interrupted at call 600 resumes without re-spending.

**The budget guard fails closed.** The client tracks
`X-Nansen-Credits-Remaining` from every response, reserves credits under a mutex
before a call goes out, and refuses anything that would drop the balance below
`--credit-floor` (default 100). Endpoints costing more than 1 credit are refused
outright unless `--allow-expensive` is passed. Until a live response establishes
the real balance, the worker pool stays serialized rather than letting eight
workers race out the door blind.

**Every call is receipted.** `out/calls.jsonl` records one line per call with
its endpoint, status, credit cost, remaining balance, and Nansen's own
`X-Request-Id`, so a run can be reconciled call-for-call against the dashboard.

## What it computes

**Cluster collapse.** For each token, take the seed wallets that bought it, and
union two wallets whenever they share a `First Funder`. `N buyers → M clusters`
is the headline. Only `First Funder` counts as a funding edge — the
`Deployed Program` / `Deployed Contract` relations that also come back from
`related-wallets` are contract artefacts and would merge unrelated wallets into
one meaningless blob. A wallet with no funder data is its own cluster: missing
data never manufactures a collapse.

**Exit signal.** `*_net_flow_usd` is net flow *for addresses carrying that
label*, so a positive `exchange_net_flow_usd` means value moving into exchange
addresses. Smart traders net buying while exchanges net receive is accumulation
into someone else's distribution.

**Verdict.** One categorical label per token. On independence: `THIN` when every
buyer shares one funder, `CONCENTRATED` when some of them do, `CONFIRMED` when
no two buyers share a funder *and* the exit check ran clean. The exit signal
overlays: `CONFIRMED` becomes `DISTRIBUTING`, and `THIN`/`CONCENTRATED` become
`BOTH`.

Two verdicts exist to stop missing data from passing a check it never faced.
`UNVERIFIED`: some buyers' funders were never fetched, so "no shared funder"
is unproven. `INDEPENDENT`: the buyers are independent, but no exchange
address has ever touched the token, so there was no exit to check — its
exchange flow renders as `—`, never as a reassuring `+$0`.

A token also needs at least `--min-signal-usd` (default $1,000) of smart-money
buying before the tool will clear it. Four wallets spending $156 between them
is noise, and a clean verdict on noise reads like an endorsement. The floor
only withholds that clean verdict: a shared funder that was actually found
(`THIN`, `CONCENTRATED`, `BOTH`) is reported however little was bought.

Every verdict renders next to the raw signed flow numbers, so you can disagree
with the label by reading its inputs. Tokens with fewer than three buyers carry
no independence verdict — but if their exit signal fires they still appear,
because one wallet accumulating into exchange outflows is as damning as five.

## Flags

| flag | default | meaning |
| --- | --- | --- |
| `--dry-run` | | print the plan and credit estimate, exit without any HTTP |
| `--offline` | | run from cache only; a cache miss is a hard error |
| `--yes` | | skip the confirmation prompt before spending credits |
| `--credit-floor` | 100 | refuse calls that would drop the balance below this |
| `--credits-remaining` | -1 | prime the known balance; -1 means unknown until the first response |
| `--allow-expensive` | | permit endpoints costing more than 1 credit |
| `--concurrency` | 8 | worker pool size |
| `--max-wallets` | 0 | cap enrichment to the N highest-value seed wallets (0 = all) |
| `--min-signal-usd` | 1000 | smart-money buy volume a token needs before its buyers are judged |
| `--rate-per-sec` / `--rate-per-min` | 12 / 250 | kept under the Free tier's 15 / 300 |

## Layout

```
cmd/scorecard/      CLI, terminal table, HTML report
internal/nansen/    HTTP client: cache, budget guard, rate limit, call log
internal/pipeline/  seed -> wallet enrichment -> token enrichment
internal/score/     pure scoring functions
docs/DESIGN.md      verified endpoint facts and the credit budget
docs/DEMO.md        demo recording script
scripts/probe.sh    probe one endpoint and save the response as a fixture
```

`data/`, `out/`, and `.env` are gitignored — the cache and fixtures hold paid
API responses.

## Notes

- The API key goes in the `NANSEN_API_KEY` environment variable. The binary
  does not read `.env` itself; source it first if you keep one.
- All Nansen endpoints are `POST`, including the ones that read like `GET`s, and
  the auth header is `apikey`, not `Authorization`.
- `related-wallets` rejects `chain: "all"` even though `pnl-summary` accepts it,
  so it is called once per (wallet, chain) pair seen in the seed.
