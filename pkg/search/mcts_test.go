package search

import (
	"math/rand"
	"testing"
	"time"

	"github.com/Thanhphan1147/root-bot/pkg/eval"
	"github.com/Thanhphan1147/root-mn/pkg/root"
)

func midGame(t *testing.T, seed uint64, plies int) *root.Game {
	t.Helper()
	g := root.NewGame([]root.Faction{root.MC, root.ED}, root.MC, seed)
	root.BeginSetup(g)
	rng := rand.New(rand.NewSource(int64(seed)))
	for i := 0; i < plies; i++ {
		acts := g.LegalActions()
		if len(acts) == 0 || len(g.Winner) > 0 {
			break
		}
		if err := g.Apply(acts[rng.Intn(len(acts))]); err != nil {
			break
		}
	}
	return g
}

// TestChooseReturnsLegal checks the search picks a legal action and does not
// mutate the position it was handed.
func TestChooseReturnsLegal(t *testing.T) {
	profs := []string{"material", "eyrie"}
	for _, name := range profs {
		h, _ := eval.ProfileByName(name)
		m := MCTS{Eval: h, Rng: rand.New(rand.NewSource(1)), Sims: 40}
		g := midGame(t, 3, 60)
		before := g.Clone()
		f := g.Actor()
		mv := m.Choose(g, f)
		if mv.ID == "" {
			t.Fatalf("%s: empty action", name)
		}
		legal := false
		for _, a := range g.LegalActions() {
			if a.ID == mv.ID {
				legal = true
				break
			}
		}
		if !legal {
			t.Fatalf("%s: chose illegal action %q", name, mv.ID)
		}
		if !gamesEqual(before, g) {
			t.Fatalf("%s: Choose mutated the game", name)
		}
	}
}

func gamesEqual(a, b *root.Game) bool {
	return a.Phase == b.Phase && a.Current == b.Current && a.Round == b.Round && a.RngSeed == b.RngSeed
}

// BenchmarkChoose measures MCTS throughput for a fixed budget. Use it to catch
// search regressions before they reach a game loop.
func BenchmarkChoose(b *testing.B) {
	h, _ := eval.ProfileByName("eyrie")
	g := root.NewGame([]root.Faction{root.MC, root.ED}, root.MC, 5)
	root.BeginSetup(g)
	rng := rand.New(rand.NewSource(5))
	for i := 0; i < 80; i++ {
		acts := g.LegalActions()
		if len(acts) == 0 || len(g.Winner) > 0 {
			break
		}
		if err := g.Apply(acts[rng.Intn(len(acts))]); err != nil {
			break
		}
	}
	f := g.Actor()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := MCTS{Eval: h, Rng: rand.New(rand.NewSource(int64(i) + 1)), Sims: 200, PriorDepth: 0}
		_ = m.Choose(g, f)
	}
}

// TestChooseIsBounded is a smoke test that a modest budget finishes quickly, so
// runaway search cannot take down the host.
func TestChooseIsBounded(t *testing.T) {
	h, _ := eval.ProfileByName("eyrie")
	m := MCTS{Eval: h, Rng: rand.New(rand.NewSource(2)), Sims: 100, Worlds: 3}
	g := midGame(t, 5, 80)
	start := time.Now()
	_ = m.Choose(g, g.Actor())
	if el := time.Since(start); el > 10*time.Second {
		t.Fatalf("100 sims x 3 worlds took %s; search is not bounded", el)
	}
}
