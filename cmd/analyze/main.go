// Command analyze plays Eyrie-vs-Marquise games and reports how they are won or
// lost: final VP, roosts, turmoils, and the action mix each side chooses.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"github.com/Thanhphan1147/root-bot/pkg/bot"
	"github.com/Thanhphan1147/root-mn/pkg/root"
)

func main() {
	games := flag.Int("games", 10, "games to analyze")
	maxSteps := flag.Int("max", 6000, "max actions per game")
	mcSpec := flag.String("mc", "greedy:material", "MC bot")
	edSpec := flag.String("ed", "greedy:material", "ED bot")
	sims := flag.Int("sims", 0, "MCTS sims")
	flag.Parse()

	rng := rand.New(rand.NewSource(1))
	mcKinds := map[string]int{}
	edKinds := map[string]int{}
	leaders := map[string]int{}
	sumEDRoost, sumMCRoost, sumTurmoil := 0, 0, 0
	edWins := 0
	for i := 0; i < *games; i++ {
		mc := bot.Make(*mcSpec, int64(i)+1, *sims, 0)
		ed := bot.Make(*edSpec, int64(i)+1000, *sims, 0)
		g := root.NewGame([]root.Faction{root.MC, root.ED}, root.MC, rng.Uint64())
		root.BeginSetup(g)
		for steps := 0; steps < *maxSteps; steps++ {
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
		}
		for _, e := range g.Log {
			if e.Actor == root.MC {
				mcKinds[e.Kind]++
			} else {
				edKinds[e.Kind]++
			}
			if e.Kind == "turmoil" {
				sumTurmoil++
			}
			if e.Kind == "leader" {
				fs := strings.Fields(e.Text)
				if len(fs) >= 3 {
					leaders[fs[len(fs)-1]]++
				}
			}
		}
		sumEDRoost += countByOwner(g, root.ED)
		sumMCRoost += countByOwner(g, root.MC)
		if len(g.Winner) > 0 && g.Winner[0] == root.ED {
			edWins++
		}
	}
	fmt.Printf("ED=%s  MC=%s  games=%d  ED wins=%d\n", *edSpec, *mcSpec, *games, edWins)
	fmt.Printf("avg ED roosts %.1f  avg MC roosts %.1f  avg ED turmoils %.2f\n",
		avg(sumEDRoost, *games), avg(sumMCRoost, *games), avg(sumTurmoil, *games))
	printKinds("MC actions", mcKinds)
	printKinds("ED actions", edKinds)
	printKinds("ED leaders chosen", leaders)
}

func countByOwner(g *root.Game, f root.Faction) int {
	n := 0
	for _, c := range g.Clearings {
		for _, b := range c.Buildings {
			if b.Owner == f {
				n++
			}
		}
	}
	return n
}

func printKinds(title string, m map[string]int) {
	type kv struct {
		k string
		v int
	}
	var xs []kv
	for k, v := range m {
		xs = append(xs, kv{k, v})
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i].v > xs[j].v })
	fmt.Printf("%s:", title)
	for _, x := range xs {
		fmt.Printf(" %s=%d", x.k, x.v)
	}
	fmt.Println()
}

func avg(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}
