// Command diag plays MC-vs-ED with configurable bots and reports how much each
// faction actually produces, to locate balance or policy problems.
package main

import (
	"flag"
	"fmt"
	"math/rand"

	"github.com/Thanhphan1147/root-bot/pkg/bot"
	"github.com/Thanhphan1147/root-mn/pkg/root"
)

func main() {
	games := flag.Int("games", 10, "games to play")
	maxSteps := flag.Int("max", 6000, "max actions per game")
	mcSpec := flag.String("mc", "greedy:full", "MC bot spec")
	edSpec := flag.String("ed", "random", "ED bot spec")
	sims := flag.Int("sims", 200, "MCTS simulations per move")
	rollout := flag.Int("rollout", 0, "MCTS rollout depth")
	flag.Parse()

	mcBot := bot.Make(*mcSpec, 1, *sims, *rollout)
	edBot := bot.Make(*edSpec, 2, *sims, *rollout)
	rng := rand.New(rand.NewSource(1))

	mcWins, edWins, draws := 0, 0, 0
	sumMC, sumED, sumTurmoil := 0, 0, 0
	for i := 0; i < *games; i++ {
		g := root.NewGame([]root.Faction{root.MC, root.ED}, root.MC, rng.Uint64())
		root.BeginSetup(g)
		for steps := 0; steps < *maxSteps; steps++ {
			if len(g.Winner) > 0 {
				break
			}
			if len(g.LegalActions()) == 0 {
				break
			}
			f := g.Actor()
			mover := edBot
			if f == root.MC {
				mover = mcBot
			}
			mv := mover.Choose(g, f)
			if mv.ID == "" {
				break
			}
			if err := g.Apply(mv); err != nil {
				break
			}
		}
		mc, ed := g.Players[root.MC], g.Players[root.ED]
		sumMC += mc.VP
		sumED += ed.VP
		turmoils := 0
		for _, e := range g.Log {
			if e.Kind == "turmoil" {
				turmoils++
			}
		}
		sumTurmoil += turmoils
		switch {
		case len(g.Winner) == 0:
			draws++
		case g.Winner[0] == root.MC:
			mcWins++
		case g.Winner[0] == root.ED:
			edWins++
		default:
			draws++
		}
		fmt.Printf("g%02d r=%-3d MC vp=%-3d items=%-2d build=%d/%d/%d wood=%-2d | ED vp=%-3d roosts=%d turmoil=%d win=%v\n",
			i, g.Round, mc.VP, len(mc.CraftedItems),
			cb(g, root.MC, "sawmill"), cb(g, root.MC, "workshop"), cb(g, root.MC, "recruiter"), wood(g),
			ed.VP, cb(g, root.ED, "roost"), turmoils, g.Winner)
	}
	fmt.Printf("\nMC(%s) wins %d | ED(%s) wins %d | draws %d | avg MC vp %.1f, ED vp %.1f, ED turmoils %.1f\n",
		mcBot.Name(), mcWins, edBot.Name(), edWins, draws,
		float64(sumMC)/float64(*games), float64(sumED)/float64(*games), float64(sumTurmoil)/float64(*games))
}

func cb(g *root.Game, f root.Faction, typ string) int {
	n := 0
	for _, c := range g.Clearings {
		for _, b := range c.Buildings {
			if b.Owner == f && b.Type == typ {
				n++
			}
		}
	}
	return n
}

func wood(g *root.Game) int {
	n := 0
	for _, c := range g.Clearings {
		n += c.Wood
	}
	return n
}
