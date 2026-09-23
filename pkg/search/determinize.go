package search

import (
	"math/rand"

	"github.com/Thanhphan1147/root-mn/pkg/root"
)

// Determinize returns a copy of g with the hidden zone resampled for viewer: the
// opponent's hand and the deck are one shuffled pool split back into the same
// counts. Own hand, discard, crafted cards and the Decree stay as they are,
// since those are public. The RNG seed is also resampled so battle dice and
// future shuffles differ between worlds.
//
// This is what keeps the search honest: without it the tree would read the
// opponent's actual hand, which a real player cannot see.
func Determinize(g *root.Game, viewer root.Faction, rng *rand.Rand) *root.Game {
	out := g.CloneForSearch()
	if out == nil {
		return nil
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}

	opp := opponent(g, viewer)
	vp, op := out.Players[viewer], out.Players[opp]

	// If the viewer has actually looked at the opponent's hand this turn (e.g.
	// Codebreakers), it is public information and must not be scrambled.
	if vp != nil && vp.Revealed[opp] {
		out.RngSeed = rng.Uint64()
		return out
	}
	if op == nil {
		out.RngSeed = rng.Uint64()
		return out
	}

	pool := make([]string, 0, len(op.Hand)+len(out.Deck))
	pool = append(pool, op.Hand...)
	pool = append(pool, out.Deck...)
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	n := len(op.Hand)
	op.Hand = append([]string(nil), pool[:n]...)
	out.Deck = append([]string(nil), pool[n:]...)
	out.RngSeed = rng.Uint64()
	return out
}

func opponent(g *root.Game, f root.Faction) root.Faction {
	for _, x := range g.Order {
		if x != f {
			return x
		}
	}
	return ""
}
