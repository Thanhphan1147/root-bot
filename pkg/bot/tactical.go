package bot

import (
	"github.com/Thanhphan1147/root-mn/pkg/root"
)

// Tactical wraps an inner policy and takes a winning move the moment one is
// available. Reaching the 30-VP threshold or a dominance win ends the game, so
// failing to take it (or a search that does not see it) is a pure blunder.
// Checking costs one clone+apply per legal action, which is negligible next to
// the inner search.
type Tactical struct {
	Inner Bot
}

// Name implements Bot.
func (t Tactical) Name() string { return "tactical:" + t.Inner.Name() }

// Choose implements Bot.
func (t Tactical) Choose(g *root.Game, f root.Faction) root.Action {
	for _, a := range g.LegalActions() {
		c := g.CloneForSearch()
		if err := c.ApplyFast(a); err != nil {
			continue
		}
		if wins(c, f) {
			return a
		}
	}
	return t.Inner.Choose(g, f)
}

func wins(g *root.Game, f root.Faction) bool {
	for _, w := range g.Winner {
		if w == f {
			return true
		}
	}
	return false
}
