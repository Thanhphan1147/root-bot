// Command selfplay runs bot-vs-bot games to compare play policies and, with
// -matrix, every evaluation profile against every other.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"time"

	"github.com/Thanhphan1147/root-bot/internal/arena"
	"github.com/Thanhphan1147/root-bot/pkg/bot"
	"github.com/Thanhphan1147/root-bot/pkg/eval"
)

func main() {
	games := flag.Int("games", 100, "games per match (paired games count double)")
	maxSteps := flag.Int("max", 6000, "max actions per game")
	seed := flag.Int64("seed", 1, "base seed")
	aSpec := flag.String("a", "greedy:full", "bot A spec (random | passive | greedy:<p> | mcts:<p>)")
	bSpec := flag.String("b", "greedy:material", "bot B spec")
	matrix := flag.Bool("matrix", false, "round-robin every eval profile (greedy)")
	fix := flag.Bool("fix", false, "A always plays MC, B always plays ED")
	sims := flag.Int("sims", 200, "MCTS simulations per move")
	rollout := flag.Int("rollout", 0, "MCTS rollout depth")
	flag.Parse()

	if *matrix {
		runMatrix(*games, *maxSteps, *seed)
		return
	}

	a := bot.Make(*aSpec, *seed, *sims, *rollout)
	b := bot.Make(*bSpec, *seed+1, *sims, *rollout)
	start := time.Now()
	var res arena.Result
	if *fix {
		res = arena.Play(a, b, *games, *seed, *maxSteps, false)
	} else {
		res = arena.PlayPaired(a, b, *games, *seed, *maxSteps)
	}
	el := time.Since(start)
	fmt.Printf("A=%s  B=%s\n", a.Name(), b.Name())
	fmt.Printf("%d games in %s (%.2f games/s, avg %.0f steps)\n",
		res.Games, el.Round(time.Millisecond), float64(res.Games)/el.Seconds(),
		float64(res.Steps)/float64(res.Games))
	fmt.Printf("A %d (%.1f%% +/-%.1f)  B %d (%.1f%%)  draws %d  Elo(A-B) %+.0f\n",
		res.A, pct(res.A, res.Games), 100*res.WinRateSE(),
		res.B, pct(res.B, res.Games), res.Draws, res.EloA())
}

func runMatrix(games, maxSteps int, seed int64) {
	hs := eval.Profiles
	fmt.Printf("round-robin, %d paired games/pairing, greedy; cell = row's win rate\n", games)
	fmt.Printf("%-10s", "row\\col")
	for _, h := range hs {
		fmt.Printf("%9s", h.Label)
	}
	fmt.Println()
	for i, a := range hs {
		fmt.Printf("%-10s", a.Label)
		for j, b := range hs {
			if i == j {
				fmt.Printf("%9s", "—")
				continue
			}
			botA := bot.Greedy{Eval: a, Rng: rand.New(rand.NewSource(seed + int64(i)*131 + int64(j)))}
			botB := bot.Greedy{Eval: b, Rng: rand.New(rand.NewSource(seed + int64(j)*131 + int64(i)))}
			res := arena.PlayPaired(botA, botB, games, seed+int64(i*97+j*13), maxSteps)
			fmt.Printf("%8.0f%%", pct(res.A, res.Games))
		}
		fmt.Println()
	}
}

func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return 100 * float64(n) / float64(d)
}
