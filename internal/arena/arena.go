// Package arena plays bots against each other so heuristics can be compared.
package arena

import (
	"math/rand"

	"github.com/Thanhphan1147/root-bot/pkg/bot"
	"github.com/Thanhphan1147/root-mn/pkg/root"
)

// Result summarises a set of games between two bots.
type Result struct {
	Games int
	Steps int
	A     int // games won by bot A
	B     int // games won by bot B
	Draws int // unfinished or no winner within the step cap
}

// Play runs `games` games of MC-vs-ED. When alternate is true it swaps which
// bot plays which faction each game, cancelling any first-player advantage;
// otherwise A always plays MC and B always plays ED.
func Play(a, b bot.Bot, games int, seed int64, maxSteps int, alternate bool) Result {
	rng := rand.New(rand.NewSource(seed))
	res := Result{Games: games}
	for i := 0; i < games; i++ {
		g := root.NewGame([]root.Faction{root.MC, root.ED}, root.MC, rng.Uint64())
		root.BeginSetup(g)

		aFaction, bFaction := root.MC, root.ED
		if alternate && i%2 == 1 {
			aFaction, bFaction = root.ED, root.MC
		}

		steps := 0
		for steps < maxSteps {
			if len(g.Winner) > 0 {
				break
			}
			if len(g.LegalActions()) == 0 {
				break
			}
			f := bot.Actor(g)
			mover := b
			if f == aFaction {
				mover = a
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
		res.Steps += steps

		switch {
		case len(g.Winner) == 0:
			res.Draws++
		case g.Winner[0] == aFaction:
			res.A++
		case g.Winner[0] == bFaction:
			res.B++
		default:
			res.Draws++
		}
	}
	return res
}
