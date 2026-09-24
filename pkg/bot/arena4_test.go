package bot

import (
	"testing"

	"github.com/Thanhphan1147/root-mn/pkg/root"
)

// play4 plays a full four-player game with the shipped per-faction profiles.
func play4(seed uint64, specs map[root.Faction]string) (map[root.Faction]int, []root.Faction) {
	g := root.NewGame([]root.Faction{root.MC, root.ED, root.WA, root.VB}, root.MC, seed)
	root.BeginSetup(g)
	bots := map[root.Faction]Bot{}
	for f, s := range specs {
		bots[f] = Make(s, int64(seed)+int64(len(f)), 0, 0)
	}
	for i := 0; i < 40000; i++ {
		if len(g.Winner) > 0 || len(g.LegalActions()) == 0 {
			break
		}
		f := g.Actor()
		mv := bots[f].Choose(g, f)
		if mv.ID == "" {
			break
		}
		if err := g.Apply(mv); err != nil {
			break
		}
	}
	vp := map[root.Faction]int{}
	for f := range g.Players {
		vp[f] = g.Players[f].VP
	}
	return vp, g.Winner
}

// TestFourPlayerArena is a soft benchmark: it plays four-player games with the
// custom WA/VB profiles and the generic baseline, logs the averages, and fails
// only if the custom WA profile regresses badly.
func TestFourPlayerArena(t *testing.T) {
	custom := map[root.Faction]string{root.MC: "greedy:material", root.ED: "greedy:eyrie", root.WA: "greedy:wa", root.VB: "greedy:vb"}
	base := map[root.Faction]string{root.MC: "greedy:material", root.ED: "greedy:eyrie", root.WA: "greedy:material", root.VB: "greedy:material"}

	const games = 12
	sumCustomWA, sumBaseWA := 0, 0
	for seed := uint64(1); seed <= games; seed++ {
		vp, _ := play4(seed, custom)
		sumCustomWA += vp[root.WA]
		bv, _ := play4(seed, base)
		sumBaseWA += bv[root.WA]
	}
	cWA := float64(sumCustomWA) / games
	bWA := float64(sumBaseWA) / games
	t.Logf("WA average VP over %d games: custom %.1f (wa profile) vs baseline %.1f (material)", games, cWA, bWA)
	if cWA+5 < bWA {
		t.Fatalf("custom WA profile regressed: %.1f vs baseline %.1f", cWA, bWA)
	}
}
