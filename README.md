# root-bot

Bots for 1v1 ROOT, built on the [Root Machine Notation](https://github.com/Thanhphan1147/root-mn)
engine. Goal: let a lone player face a bot in the multiplayer client, starting
with **Eyrie vs Marquise**.

> Status: playable. The framework, evaluation heuristics, search, a self-play
> harness, a serverless browser demo, and server-side bot seats in
> [root-multiplayer](https://github.com/Thanhphan1147/root-multiplayer) are in
> place. A 1v1 balance problem (below) still limits how good the games are.

## Architecture decision: one engine, per-faction evaluation

Two options were on the table:

1. **Separate rule engines per faction** (an MC-only engine, an ED-only engine).
2. **One shared rule engine, with faction-specialised evaluation and policy.**

We chose **(2)**. Reasons:

- The *rules* do not depend on which faction a bot plays; only the *strategy*
  does. Duplicating the rules doubles the surface for bugs and lets the two
  copies diverge.
- Modern engines (Stockfish, AlphaZero) separate move generation from evaluation
  and policy. "Specialisation" lives in the evaluation, not the rules.
- Self-play between factions — the whole point of the heuristic comparison — is
  trivial with one engine and needs a protocol bridge with two.
- The existing `pkg/root` engine already plays any 2–4 faction subset, so it is
  the shared substrate.

So a "faction engine" here is a **profile**: a set of evaluation weights plus
(future) a faction-specific rollout policy. `internal/eval` holds the profiles;
`internal/bot` holds the policies; `internal/search` holds the tree search.

## Layout

    cmd/bench      engine throughput benchmark
    cmd/diag       play two bots, dump each side's output (balance diagnosis)
    cmd/selfplay   bot-vs-bot matches and a heuristic round-robin
    internal/bot   policies: random, passive, greedy(1-ply), MCTS
    internal/eval  positional heuristics and profiles
    internal/search MCTS
    internal/arena self-play match runner

## Running

    go run ./cmd/selfplay -a greedy:full -b greedy:material -games 100
    go run ./cmd/selfplay -matrix -games 50          # round-robin heuristics
    go run ./cmd/selfplay -a mcts:full -b greedy:full -games 20 -sims 200
    go run ./cmd/diag -mc greedy:full -ed random -games 10

## Play in your browser

A serverless demo compiles the engine and bot to WebAssembly and runs entirely in
the page — no server:

**https://thanhphan1147.github.io/root-bot/**

Pick a side (Marquise or Eyrie) and an engine strength, then play 1v1. The bundle
is built with `bash scripts/build-web.sh` (or `make web`) into `docs/`, which
GitHub Pages serves; `cmd/botwasm` is the WASM entry point.

The same bot also plays seats on the multiplayer server (`rmn-mp add-bot ...`),
so a lone player in a room has an opponent.

## Engine work done for the bot

- `root.Game.Clone` was a JSON round trip (~440 us/op). It is now a manual deep
  copy (~5 us/op) — an **83x** speedup that makes search practical.
  `CloneForSearch` skips the action logs a search never reads.
- Added `root.Game.Actor()`: the faction that must act (the pending player during
  a deferred choice, else the current player), so bots move the right seat.

Both shipped in root-mn **v0.1.4**.

## Findings

Engine throughput (random MC-vs-ED): ~45k steps/s, ~80 games/s.

**ED dominates MC under automated play.** With random and greedy policies, ED
wins ~100% of games. MC tops out around **15 VP** while ED reliably reaches 30.

    MC(greedy:full) wins 0 | ED(random) wins 8 | avg MC vp 15.1, ED vp 31.0

Giving MC a much larger search budget did not help (500-sim MCTS still lost
100%). MC's mechanics are implemented and its build track is worth up to ~44 VP,
so this looks like a **strategy/balance** gap rather than a crash: MC builds and
crafts a little, is action- and wood-starved, and never pressures the Eyrie's
roosts, which score every Evening.

## Roadmap

1. Resolve the MC/ED imbalance (audit the Eyrie decree/turmoil economy and MC's
   action economy; add a roost-pressure term; verify against the real rules).
2. Fair search: current MCTS sees the true state. Move to determinised MCTS
   (sample the opponent's hand and deck from the public information) in both the
   server bot and the browser demo.
3. Faster search: an apply path that skips re-validation and logging, and run the
   browser MCTS in a Web Worker so deep search does not block the page.