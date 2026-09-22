// Package eval scores ROOT positions for the bot. A heuristic is a linear
// combination of features from one faction's point of view; several profiles are
// provided so they can be compared by self-play.
package eval

import "github.com/Thanhphan1147/root-mn/pkg/root"

// Evaluator scores a position from f's point of view (positive is good for f).
type Evaluator interface {
	Eval(g *root.Game, f root.Faction) float64
	Name() string
}

// Weights tunes a Heuristic. All features are "mine minus theirs" except the
// faction terms, which are absolute.
type Weights struct {
	VP       float64 // VP lead
	Warrior  float64
	Building float64
	Token    float64
	Rule     float64 // ruled clearings
	Card     float64 // cards in hand

	// Faction terms (absolute, only for the faction they belong to).
	MCWood    float64 // board wood + supply
	MCBuild   float64 // sawmills/workshops/recruiters placed
	EDRoost   float64 // roosts on the map
	EDDecree  float64 // decree cards queued
	EDLeader  float64 // a leader is seated (avoids early turmoil)
	KeepBonus float64 // Marquise keep still on the board
	EDRisk    float64 // per decree card that cannot currently be resolved
}

// Heuristic is a weighted evaluator.
type Heuristic struct {
	Label string
	W     Weights
}

// Name identifies the heuristic.
func (h Heuristic) Name() string { return h.Label }

// Eval implements Evaluator.
func (h Heuristic) Eval(g *root.Game, f root.Faction) float64 {
	if g == nil {
		return 0
	}
	if len(g.Winner) > 0 {
		for _, w := range g.Winner {
			if w == f {
				return 100000
			}
		}
		return -100000
	}
	o := opponent(g, f)
	if o == "" {
		return 0
	}
	pf, po := g.Players[f], g.Players[o]
	if pf == nil || po == nil {
		return 0
	}
	w := h.W
	s := w.VP * float64(pf.VP-po.VP)
	s += w.Warrior * float64(warriors(g, f)-warriors(g, o))
	s += w.Building * float64(buildings(g, f)-buildings(g, o))
	s += w.Token * float64(tokens(g, f)-tokens(g, o))
	s += w.Rule * float64(ruled(g, f)-ruled(g, o))
	s += w.Card * float64(len(pf.Hand)-len(po.Hand))
	s += factionValue(g, f, w) - factionValue(g, o, w)
	return s
}

func factionValue(g *root.Game, f root.Faction, w Weights) float64 {
	p := g.Players[f]
	if p == nil {
		return 0
	}
	var v float64
	switch f {
	case root.MC:
		v += w.MCWood * float64(boardWood(g)+p.WoodSupply)
		v += w.MCBuild * float64(totalBuildings(g, root.MC))
		if keepOnBoard(g) {
			v += w.KeepBonus
		}
	case root.ED:
		v += w.EDRoost * float64(totalBuildings(g, root.ED))
		v += w.EDDecree * float64(decreeSize(p))
		if p.Leader != "" {
			v += w.EDLeader
		}
		v -= w.EDRisk * edDecreeRisk(g)
	}
	return v
}

// --- feature extraction ---

func opponent(g *root.Game, f root.Faction) root.Faction {
	for _, x := range g.Order {
		if x != f {
			return x
		}
	}
	return ""
}

func warriors(g *root.Game, f root.Faction) int {
	n := 0
	for _, c := range g.Clearings {
		n += c.Warriors[f]
	}
	return n
}

func buildings(g *root.Game, f root.Faction) int { return totalBuildings(g, f) }

func totalBuildings(g *root.Game, f root.Faction) int {
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

func tokens(g *root.Game, f root.Faction) int {
	n := 0
	for _, c := range g.Clearings {
		for _, t := range c.Tokens {
			if t.Owner == f {
				n++
			}
		}
	}
	return n
}

func ruled(g *root.Game, f root.Faction) int {
	n := 0
	for id := range g.Clearings {
		if g.Rules(f, id) {
			n++
		}
	}
	return n
}

func boardWood(g *root.Game) int {
	n := 0
	for _, c := range g.Clearings {
		n += c.Wood
	}
	return n
}

func keepOnBoard(g *root.Game) bool {
	for _, c := range g.Clearings {
		for _, t := range c.Tokens {
			if t.Owner == root.MC && t.Type == "keep" {
				return true
			}
		}
	}
	return false
}

func decreeSize(p *root.Player) int {
	n := 0
	for _, col := range p.Decree {
		n += len(col)
	}
	return n
}

// --- profiles ---

// Profiles lists the heuristics compared by the self-play harness.
var Profiles = []Heuristic{
	{Label: "vp", W: Weights{VP: 1}},
	{Label: "material", W: Weights{VP: 1, Warrior: 0.15, Building: 0.4, Token: 0.4, Rule: 0.2}},
	{
		Label: "full",
		W: Weights{
			VP: 1, Warrior: 0.15, Building: 0.4, Token: 0.4, Rule: 0.25, Card: 0.2,
			MCWood: 0.08, MCBuild: 0.5, EDRoost: 0.6, EDDecree: 0.1, EDLeader: 0.5, KeepBonus: 1.0,
			EDRisk: 2.0,
		},
	},
	{
		Label: "tuned",
		W: Weights{
			VP: 1.0, Warrior: 0.12, Building: 0.35, Token: 0.35, Rule: 0.3, Card: 0.25,
			MCWood: 0.05, MCBuild: 0.45, EDRoost: 0.7, EDDecree: 0.05, EDLeader: 0.6, KeepBonus: 1.2,
			EDRisk: 3.0,
		},
	},
}

// ProfileByName returns a profile by label.
func ProfileByName(name string) (Heuristic, bool) {
	for _, p := range Profiles {
		if p.Label == name {
			return p, true
		}
	}
	return Heuristic{}, false
}

// --- Eyrie decree safety ---

// edDecreeRisk counts Decree cards that cannot currently be resolved, which
// would force the Eyrie into Turmoil.
func edDecreeRisk(g *root.Game) float64 {
	p := g.Players[root.ED]
	if p == nil {
		return 0
	}
	risk := 0.0
	for col, cards := range p.Decree {
		for _, id := range cards {
			if !decreeCardResolvable(g, col, decreeSuit(id)) {
				risk++
			}
		}
	}
	return risk
}

func decreeSuit(id string) root.Suit {
	if id == "VIZIER" {
		return root.Bird
	}
	if c, ok := root.Card(id); ok {
		return c.Suit
	}
	return ""
}

func suitMatches(have, want root.Suit) bool {
	return have == want || have == root.Bird || want == root.Bird
}

// decreeCardResolvable approximates whether a Decree card of a given column and
// suit has a legal target on the current board.
func decreeCardResolvable(g *root.Game, col string, suit root.Suit) bool {
	switch col {
	case "RECRUIT":
		for cid, c := range g.Clearings {
			if hasRoost(g, cid) && suitMatches(c.Suit, suit) {
				return true
			}
		}
	case "MOVE":
		for _, c := range g.Clearings {
			if suitMatches(c.Suit, suit) && c.Warriors[root.ED] > 0 && len(c.Adj) > 0 {
				return true
			}
		}
	case "BATTLE":
		for _, c := range g.Clearings {
			if !suitMatches(c.Suit, suit) || c.Warriors[root.ED] == 0 {
				continue
			}
			for f, n := range c.Warriors {
				if f != root.ED && n > 0 {
					return true
				}
			}
			for _, b := range c.Buildings {
				if b.Owner != root.ED {
					return true
				}
			}
			for _, t := range c.Tokens {
				if t.Owner != root.ED {
					return true
				}
			}
		}
	case "BUILD":
		for cid, c := range g.Clearings {
			if suitMatches(c.Suit, suit) && g.Rules(root.ED, cid) && !hasRoost(g, cid) && c.FreeSlots() > 0 {
				return true
			}
		}
	}
	return false
}

func hasRoost(g *root.Game, c string) bool {
	for _, b := range g.Clearings[c].Buildings {
		if b.Owner == root.ED && b.Type == "roost" {
			return true
		}
	}
	return false
}
