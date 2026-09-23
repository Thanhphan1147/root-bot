// Command suite runs a fixed benchmark of candidate bots against a fixed
// reference, so policy changes can be compared on a stable, low-variance
// signal. Each candidate is a per-side pair (MC spec, ED spec); --games pairs
// means 2*games games per candidate.
package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/Thanhphan1147/root-bot/internal/arena"
	"github.com/Thanhphan1147/root-bot/pkg/bot"
)

type candidate struct {
	name string
	mc   string
	ed   string
}

func main() {
	games := flag.Int("games", 40, "pairs per candidate (2*games games)")
	sims := flag.Int("sims", 400, "MCTS simulations per move")
	rollout := flag.Int("rollout", 0, "MCTS rollout depth")
	seed := flag.Int64("seed", 1, "base seed")
	refMC := flag.String("refmc", "greedy:material", "reference MC spec")
	refED := flag.String("refed", "greedy:eyrie", "reference ED spec")
	only := flag.String("only", "", "comma list of candidate names to run (default all)")
	maxSteps := flag.Int("max", 6000, "max actions per game")
	flag.Parse()

	ref := bot.MakePair(*refMC, *refED, *seed+5000, 0, 0)
	cands := []candidate{
		{"shipped", "greedy:material", "greedy:eyrie"},
		{"mcts", "mcts:material", "mcts:eyrie"},
	}
	if *only != "" {
		want := map[string]bool{}
		for _, n := range strings.Split(*only, ",") {
			want[strings.TrimSpace(n)] = true
		}
		var keep []candidate
		for _, c := range cands {
			if want[c.name] {
				keep = append(keep, c)
			}
		}
		cands = keep
	}

	fmt.Printf("reference: %s / %s   (%d pairs = %d games per candidate)\n\n",
		*refMC, *refED, *games, 2*(*games))
	fmt.Printf("%-12s %8s %8s %8s %8s %6s\n", "candidate", "wins", "losses", "draws", "win%", "elo")
	for i, c := range cands {
		cand := bot.MakePair(c.mc, c.ed, *seed+int64(i)*17, *sims, *rollout)
		res := arena.PlayPaired(cand, ref, *games, *seed+int64(i)*101, *maxSteps)
		fmt.Printf("%-12s %8d %8d %8d %7.1f%% %+6.0f\n",
			c.name, res.A, res.B, res.Draws,
			100*res.WinRateA(), res.EloA())
	}
}
