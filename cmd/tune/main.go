// Command tune compares Eyrie evaluation profiles against a fixed Marquise
// opponent, reporting win rate, VP, and turmoils per game.
package main

import (
	"flag"
	"fmt"
	"math/rand"

	"github.com/Thanhphan1147/root-bot/pkg/bot"
	"github.com/Thanhphan1147/root-mn/pkg/root"
)

type stats struct {
	games, wins, draws, turmoils, steps int
	sumVP, sumOppVP                     int
}

func main() {
	games := flag.Int("games", 20, "games per variation")
	maxSteps := flag.Int("max", 6000, "max actions per game")
	opp := flag.String("mc", "greedy:tuned", "MC opponent spec")
	sims := flag.Int("sims", 0, "MCTS sims if a spec uses mcts")
	flag.Parse()

	profiles := []string{"tuned", "ed-lean", "ed-safe"}
	if flag.NArg() > 0 {
		profiles = flag.Args()
	}
	fmt.Printf("MC opponent: %s   (%d games each)\n", *opp, *games)
	for _, prof := range profiles {
		st := play(*games, *maxSteps, "greedy:"+prof, *opp, *sims)
		fmt.Printf("%-10s  ED win %3.0f%%  ED vp %5.1f  MC vp %5.1f  turmoils %4.2f  len %3.0f\n",
			prof, pct(st.wins, st.games), avg(st.sumVP, st.games), avg(st.sumOppVP, st.games),
			avg(st.turmoils, st.games), avg(st.steps, st.games))
	}
}

func play(games, maxSteps int, edSpec, mcSpec string, sims int) stats {
	rng := rand.New(rand.NewSource(1))
	var st stats
	st.games = games
	for i := 0; i < games; i++ {
		ed := bot.Make(edSpec, int64(i)+1, sims, 0)
		mc := bot.Make(mcSpec, int64(i)+1000, sims, 0)
		g := root.NewGame([]root.Faction{root.MC, root.ED}, root.MC, rng.Uint64())
		root.BeginSetup(g)
		steps := 0
		for steps < maxSteps {
			if len(g.Winner) > 0 || len(g.LegalActions()) == 0 {
				break
			}
			f := g.Actor()
			mover := ed
			if f == root.MC {
				mover = mc
			}
			mv := mover.Choose(g, f)
			if mv.ID == "" {
				break
			}
			if err := g.Apply(mv); err != nil {
				break
			}
			steps++
		}
		st.steps += steps
		st.sumVP += g.Players[root.ED].VP
		st.sumOppVP += g.Players[root.MC].VP
		for _, e := range g.Log {
			if e.Kind == "turmoil" {
				st.turmoils++
			}
		}
		if len(g.Winner) > 0 {
			switch g.Winner[0] {
			case root.ED:
				st.wins++
			case root.MC:
			default:
				st.draws++
			}
		}
	}
	return st
}

func avg(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return 100 * float64(n) / float64(d)
}
