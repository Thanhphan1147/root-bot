// Package arena plays bots against each other so policies can be compared.
package arena

import (
	"math"
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

// Decided is the number of games with a winner (draws excluded).
func (r Result) Decided() int { return r.A + r.B }

// WinRateA is A's share of the decided games.
func (r Result) WinRateA() float64 {
	d := r.Decided()
	if d == 0 {
		return 0.5
	}
	return float64(r.A) / float64(d)
}

// WinRateSE is the standard error of WinRateA over all games, so callers can
// judge whether a difference is real rather than noise.
func (r Result) WinRateSE() float64 {
	n := r.Games
	if n == 0 {
		return 0
	}
	p := float64(r.A) / float64(n)
	return math.Sqrt(p*(1-p)/float64(n))
}

// EloA estimates A's Elo advantage from its win rate.
func (r Result) EloA() float64 {
	p := r.WinRateA()
	if p <= 0 {
		return -800
	}
	if p >= 1 {
		return 800
	}
	return -400 * math.Log10(1/p-1)
}

// Play runs `games` games of MC-vs-ED. When alternate is true it swaps which
// bot plays which faction each game, cancelling any first-player advantage;
// otherwise A always plays MC and B always plays ED. Each game gets a fresh
// deal.
func Play(a, b bot.Bot, games int, seed int64, maxSteps int, alternate bool) Result {
	rng := rand.New(rand.NewSource(seed))
	res := Result{Games: games}
	for i := 0; i < games; i++ {
		aFaction, bFaction := root.MC, root.ED
		if alternate && i%2 == 1 {
			aFaction, bFaction = root.ED, root.MC
		}
		res.add(playOne(a, b, aFaction, bFaction, rng.Uint64(), maxSteps))
	}
	return res
}

// PlayPaired runs `games` pairs of games. Both games in a pair share one deal
// (so neither bot gets the better hand), and the bots swap factions between the
// two games (so neither bot gets the better seat). This is the low-variance
// comparison to use when ranking policies.
func PlayPaired(a, b bot.Bot, games int, seed int64, maxSteps int) Result {
	rng := rand.New(rand.NewSource(seed))
	res := Result{Games: 2 * games}
	for i := 0; i < games; i++ {
		deal := rng.Uint64()
		res.add(playOne(a, b, root.MC, root.ED, deal, maxSteps))
		res.add(playOne(a, b, root.ED, root.MC, deal, maxSteps))
	}
	return res
}

// playOne plays a single game. aFaction/bFaction decide which bot sits where.
func playOne(a, b bot.Bot, aFaction, bFaction root.Faction, deal uint64, maxSteps int) Result {
	g := root.NewGame([]root.Faction{root.MC, root.ED}, root.MC, deal)
	root.BeginSetup(g)

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

	r := Result{Steps: steps}
	switch {
	case len(g.Winner) == 0:
		r.Draws = 1
	case g.Winner[0] == aFaction:
		r.A = 1
	case g.Winner[0] == bFaction:
		r.B = 1
	default:
		r.Draws = 1
	}
	return r
}

func (r *Result) add(o Result) {
	r.Steps += o.Steps
	r.A += o.A
	r.B += o.B
	r.Draws += o.Draws
}
