// Command bench measures raw engine throughput with random self-play, to see
// whether the rules engine is fast enough for search and heuristic evaluation.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/Thanhphan1147/root-mn/pkg/root"
)

func main() {
	games := flag.Int("games", 300, "number of games")
	maxSteps := flag.Int("max", 6000, "max actions per game")
	pair := flag.String("pair", "MC,ED", "comma-separated factions")
	flag.Parse()

	factions := parseFactions(*pair)
	rng := rand.New(rand.NewSource(1))

	steps, finished, draws := 0, 0, 0
	wins := map[root.Faction]int{}
	lengths := make([]int, 0, *games)

	start := time.Now()
	for i := 0; i < *games; i++ {
		g := root.NewGame(factions, factions[0], rng.Uint64())
		root.BeginSetup(g)
		n := 0
		for ; n < *maxSteps; n++ {
			if w, ok := g.WinnerFaction(); ok {
				wins[w]++
				finished++
				break
			}
			acts := g.LegalActions()
			if len(acts) == 0 {
				break
			}
			if err := g.Apply(acts[rng.Intn(len(acts))]); err != nil {
				panic(err)
			}
			steps++
		}
		if n >= *maxSteps {
			draws++
		}
		lengths = append(lengths, n)
	}
	el := time.Since(start)

	sort.Ints(lengths)
	median := lengths[len(lengths)/2]
	fmt.Printf("factions=%v games=%d\n", factions, *games)
	fmt.Printf("elapsed=%s  steps=%d  %.0f steps/s  %.1f games/s\n", el.Round(time.Millisecond), steps, float64(steps)/el.Seconds(), float64(*games)/el.Seconds())
	fmt.Printf("finished=%d draws=%d medianLen=%d wins=%v\n", finished, draws, median, wins)
}

func parseFactions(s string) []root.Faction {
	var out []root.Faction
	cur := ""
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if cur != "" {
				out = append(out, root.Faction(cur))
			}
			cur = ""
			continue
		}
		cur += string(s[i])
	}
	if len(out) < 2 {
		panic("need at least two factions")
	}
	return out
}
