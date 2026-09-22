// Command tune compares evaluation profiles for one side against a fixed
// opponent, reporting win rate, VP, and turmoils so profiles can be ranked by
// winrate and points.
package main

import (
	"flag"
	"fmt"
	"math/rand"

	"github.com/Thanhphan1147/root-bot/pkg/bot"
	"github.com/Thanhphan1147/root-mn/pkg/root"
)

type stats struct {
	games, edWins, mcWins, draws, turmoils, steps int
	sumEDVP, sumMCVP                              int
}

func main() {
	games := flag.Int("games", 24, "games per variation")
	maxSteps := flag.Int("max", 6000, "max actions per game")
	side := flag.String("side", "ed", "which side's profiles to vary: ed or mc")
	opp := flag.String("opp", "", "fixed opponent spec (default: the shipped bot for the other side)")
	sims := flag.Int("sims", 0, "MCTS sims if a spec uses mcts")
	spec := flag.String("spec", "greedy", "bot kind for varied profiles: greedy or mcts")
	flag.Parse()

	profiles := []string{"eyrie", "tuned", "thin1", "balanced"}
	if *side == "mc" {
		profiles = []string{"material", "mc1", "mc2", "mc3"}
	}
	if flag.NArg() > 0 {
		profiles = flag.Args()
	}

	mcSpec, edSpec := "greedy:eyrie", "greedy:material"
	if *side == "mc" {
		mcSpec = *spec + ":material" // placeholder, replaced per profile
		if *spec != "" {
			mcSpec = ""
		}
	} else {
		edSpec = ""
	}
	if *opp != "" {
		if *side == "ed" {
			mcSpec = *opp
		} else {
			edSpec = *opp
		}
	}
	if mcSpec == "" {
		mcSpec = "greedy:material"
	}
	if edSpec == "" {
		edSpec = "greedy:eyrie"
	}

	fmt.Printf("varying %s profiles; opponent: mc=%s ed=%s  (%d games each)\n", *side, mcSpec, edSpec, *games)
	for _, prof := range profiles {
		vs := *side
		st := play(*games, *maxSteps, *side, *spec, prof, mcSpec, edSpec, *sims)
		if vs == "mc" {
			fmt.Printf("%-10s  MC win %3.0f%%  MC vp %5.1f  ED vp %5.1f  ED turmoils %4.2f\n",
				prof, pct(st.mcWins, st.games), avg(st.sumMCVP, st.games), avg(st.sumEDVP, st.games), avg(st.turmoils, st.games))
		} else {
			fmt.Printf("%-10s  ED win %3.0f%%  ED vp %5.1f  MC vp %5.1f  ED turmoils %4.2f\n",
				prof, pct(st.edWins, st.games), avg(st.sumEDVP, st.games), avg(st.sumMCVP, st.games), avg(st.turmoils, st.games))
		}
	}
}

// play varies one side's profile. side is "ed" or "mc"; kind is greedy or mcts.
func play(games, maxSteps int, side, kind, prof, mcSpec, edSpec string, sims int) stats {
	rng := rand.New(rand.NewSource(1))
	var st stats
	st.games = games
	for i := 0; i < games; i++ {
		mcName, edName := mcSpec, edSpec
		if side == "ed" {
			edName = kind + ":" + prof
		} else {
			mcName = kind + ":" + prof
		}
		ed := bot.Make(edName, int64(i)+1, sims, 0)
		mc := bot.Make(mcName, int64(i)+1000, sims, 0)
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
		st.sumEDVP += g.Players[root.ED].VP
		st.sumMCVP += g.Players[root.MC].VP
		for _, e := range g.Log {
			if e.Kind == "turmoil" {
				st.turmoils++
			}
		}
		if len(g.Winner) > 0 {
			switch g.Winner[0] {
			case root.ED:
				st.edWins++
			case root.MC:
				st.mcWins++
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
