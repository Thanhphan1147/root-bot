// Package bot holds play policies. A Bot chooses an action for a faction; it
// must not mutate the game it is handed.
package bot

import (
	"math/rand"
	"strings"

	"github.com/Thanhphan1147/root-bot/pkg/eval"
	"github.com/Thanhphan1147/root-bot/pkg/search"
	"github.com/Thanhphan1147/root-mn/pkg/root"
)

// Bot chooses an action for faction f in game g.
type Bot interface {
	Name() string
	Choose(g *root.Game, f root.Faction) root.Action
}

// Actor returns the faction that must act now: the pending player when the
// engine is waiting on a deferred choice, otherwise the turn player.
func Actor(g *root.Game) root.Faction { return g.Actor() }

// Random picks a legal action uniformly.
type Random struct{ Rng *rand.Rand }

// Name implements Bot.
func (b Random) Name() string { return "random" }

// Choose implements Bot.
func (b Random) Choose(g *root.Game, f root.Faction) root.Action {
	acts := g.LegalActions()
	if len(acts) == 0 {
		return root.Action{}
	}
	return acts[b.Rng.Intn(len(acts))]
}

// Greedy plays the action whose immediate position the evaluator likes most.
type Greedy struct {
	Eval eval.Evaluator
	Rng  *rand.Rand
}

// Name implements Bot.
func (b Greedy) Name() string { return "greedy:" + b.Eval.Name() }

// Choose implements Bot.
func (b Greedy) Choose(g *root.Game, f root.Faction) root.Action {
	acts := g.LegalActions()
	if len(acts) == 0 {
		return root.Action{}
	}
	best := acts[0]
	bestScore := -1e18
	ties := 0
	for i := range acts {
		c := g.CloneForSearch()
		if err := c.ApplyFast(acts[i]); err != nil {
			continue
		}
		s := b.Eval.Eval(c, f)
		if s > bestScore {
			bestScore, best, ties = s, acts[i], 1
		} else if s == bestScore {
			ties++
			if b.Rng != nil && b.Rng.Intn(ties) == 0 {
				best = acts[i]
			}
		}
	}
	return best
}

// Passive ends its turn as soon as it can, otherwise it stalls on the first
// legal action. Useful as a null opponent in diagnostics.
type Passive struct{}

// Name implements Bot.
func (b Passive) Name() string { return "passive" }

// Choose implements Bot.
func (b Passive) Choose(g *root.Game, f root.Faction) root.Action {
	acts := g.LegalActions()
	if len(acts) == 0 {
		return root.Action{}
	}
	for _, a := range acts {
		if a.Kind == "pass" || a.Kind == "battle-skip" || a.Kind == "mc-done-crafting" {
			return a
		}
	}
	return acts[0]
}

// Composite plays a different policy per faction, so one bot can sit both
// seats with the profile suited to each faction.
type Composite struct {
	ByFaction map[root.Faction]Bot
}

// Name implements Bot.
func (c Composite) Name() string {
	return "composite"
}

// Choose implements Bot.
func (c Composite) Choose(g *root.Game, f root.Faction) root.Action {
	if b, ok := c.ByFaction[f]; ok && b != nil {
		return b.Choose(g, f)
	}
	return root.Action{}
}

// MakePair builds a per-side bot: mcSpec plays the Marquise, edSpec the Eyrie.
func MakePair(mcSpec, edSpec string, seed int64, sims, rollout int) Composite {
	return MakePairWorlds(mcSpec, edSpec, seed, sims, rollout, 1, false)
}

// MakePairWorlds is MakePair with explicit MCTS determinization controls.
func MakePairWorlds(mcSpec, edSpec string, seed int64, sims, rollout, worlds int, raw bool) Composite {
	return Composite{ByFaction: map[root.Faction]Bot{
		root.MC: MakeWorlds(mcSpec, seed, sims, rollout, worlds, raw),
		root.ED: MakeWorlds(edSpec, seed+1, sims, rollout, worlds, raw),
	}}
}

// Make builds a bot from a spec: "random", "passive", "greedy:<profile>", or
// "mcts:<profile>". MCTS searches one fair, determinized world by default.
func Make(spec string, seed int64, sims, rollout int) Bot {
	return MakeWorlds(spec, seed, sims, rollout, 1, false)
}

// MakeWorlds is Make with explicit MCTS determinization controls: worlds is the
// number of sampled hidden-information worlds per decision, and raw searches the
// true state (for benchmarking against the fair bot).
func MakeWorlds(spec string, seed int64, sims, rollout, worlds int, raw bool) Bot {
	rng := rand.New(rand.NewSource(seed))
	switch {
	case spec == "random":
		return Random{Rng: rng}
	case spec == "passive":
		return Passive{}
	case strings.HasPrefix(spec, "tactical:"):
		return Tactical{Inner: MakeWorlds(strings.TrimPrefix(spec, "tactical:"), seed, sims, rollout, worlds, raw)}
	case strings.HasPrefix(spec, "greedy:"):
		if h, ok := eval.ProfileByName(strings.TrimPrefix(spec, "greedy:")); ok {
			return Greedy{Eval: h, Rng: rng}
		}
	case strings.HasPrefix(spec, "mcts:"):
		if h, ok := eval.ProfileByName(strings.TrimPrefix(spec, "mcts:")); ok {
			return search.MCTS{Eval: h, Rng: rng, Sims: sims, RolloutDepth: rollout, Worlds: worlds, Raw: raw}
		}
	}
	panic("unknown bot spec: " + spec)
}
