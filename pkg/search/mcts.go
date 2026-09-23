// Package search implements the bot's decision-making. MCTS scores positions
// with a heuristic at the rollout horizon.
package search

import (
	"math"
	"math/rand"

	"github.com/Thanhphan1147/root-bot/pkg/eval"
	"github.com/Thanhphan1147/root-mn/pkg/root"
)

// MCTS is a best-first search over the game tree. Rollouts are capped at
// RolloutDepth and finished with the evaluator, which keeps each simulation
// cheap.
//
// Decisions are made over one or more determinized worlds: hidden cards are
// resampled per world and the per-world action values are averaged. This keeps
// the search from reading the opponent's hand. Set Worlds to 1 for the cheapest
// fair search; raise it to smooth out chance and hidden information.
type MCTS struct {
	Eval         eval.Evaluator
	Rng          *rand.Rand
	Sims         int     // simulations per decision (split across worlds)
	RolloutDepth int     // random plies before evaluating
	C            float64 // UCT exploration constant
	Worlds       int     // determinizations per decision; <=0 means 1
	Raw          bool    // if true, skip determinization (search sees the truth)
}

// Name implements bot.Bot.
func (m MCTS) Name() string { return "mcts:" + m.Eval.Name() }

type aggStat struct {
	visits int
	value  float64
}

// Choose implements bot.Bot.
func (m MCTS) Choose(g *root.Game, f root.Faction) root.Action {
	if m.Sims <= 0 {
		m.Sims = 500
	}
	if m.C == 0 {
		m.C = 1.4
	}
	r := m.Rng
	if r == nil {
		r = rand.New(rand.NewSource(1))
	}

	worlds := m.Worlds
	if worlds <= 0 {
		worlds = 1
	}
	per := m.Sims / worlds
	if per <= 0 {
		per = 1
	}

	agg := map[string]*aggStat{}
	var order []string
	for w := 0; w < worlds; w++ {
		var st *root.Game
		if m.Raw {
			st = g.CloneForSearch()
		} else {
			st = Determinize(g, f, r)
		}
		if st == nil {
			continue
		}
		rootNode := &mnode{state: st, player: st.Actor()}
		rootNode.actions = st.LegalActions()
		if len(rootNode.actions) == 0 {
			continue
		}
		for i := 0; i < per; i++ {
			m.simulate(rootNode, f, r)
		}
		for _, c := range rootNode.children {
			id := c.action.ID
			a := agg[id]
			if a == nil {
				a = &aggStat{}
				agg[id] = a
				order = append(order, id)
			}
			a.visits += c.visits
			a.value += c.value
		}
	}

	if len(order) == 0 {
		acts := g.LegalActions()
		if len(acts) == 0 {
			return root.Action{}
		}
		return acts[0]
	}

	bestID, bestMean, bestVisits := "", math.Inf(-1), -1
	for _, id := range order {
		a := agg[id]
		if a.visits == 0 {
			continue
		}
		mean := a.value / float64(a.visits)
		if mean > bestMean || (mean == bestMean && a.visits > bestVisits) {
			bestMean, bestVisits, bestID = mean, a.visits, id
		}
	}
	for _, a := range g.LegalActions() {
		if a.ID == bestID {
			return a
		}
	}
	return root.Action{}
}

type mnode struct {
	state    *root.Game
	player   root.Faction
	action   root.Action // the action that produced this node
	actions  []root.Action
	children []*mnode
	visits   int
	value    float64 // sum of evaluations from the root player's perspective
	next     int
}

func (m MCTS) simulate(rootNode *mnode, rootPlayer root.Faction, r *rand.Rand) {
	n := rootNode
	path := []*mnode{n}
	for {
		if len(n.actions) == 0 || len(n.state.Winner) > 0 {
			break
		}
		if n.next < len(n.actions) {
			a := n.actions[n.next]
			n.next++
			child := n.state.CloneForSearch()
			if err := child.Apply(a); err != nil {
				continue
			}
			cn := &mnode{state: child, player: child.Actor(), actions: child.LegalActions(), action: a}
			n.children = append(n.children, cn)
			n = cn
			path = append(path, n)
			break
		}
		next := m.selectChild(n, rootPlayer)
		if next == nil {
			break
		}
		n = next
		path = append(path, n)
	}
	v := m.rollout(n.state, rootPlayer, r)
	for _, p := range path {
		p.visits++
		p.value += v
	}
}

func (m MCTS) selectChild(n *mnode, rootPlayer root.Faction) *mnode {
	var best *mnode
	bestScore := math.Inf(-1)
	logN := math.Log(float64(n.visits) + 1)
	dir := 1.0
	if n.player != rootPlayer {
		dir = -1 // the opponent minimizes the root player's value
	}
	for _, c := range n.children {
		if c.visits == 0 {
			return c
		}
		mean := c.value / float64(c.visits)
		score := dir*mean + m.C*math.Sqrt(logN/float64(c.visits))
		if score > bestScore {
			bestScore, best = score, c
		}
	}
	return best
}

// rollout plays random plies from a copy of the position, then evaluates.
func (m MCTS) rollout(g *root.Game, rootPlayer root.Faction, r *rand.Rand) float64 {
	st := g.CloneForSearch()
	depth := m.RolloutDepth
	if depth < 0 {
		depth = 0
	}
	for d := 0; d < depth; d++ {
		if len(st.Winner) > 0 {
			break
		}
		acts := st.LegalActions()
		if len(acts) == 0 {
			break
		}
		if err := st.Apply(acts[r.Intn(len(acts))]); err != nil {
			break
		}
	}
	return m.Eval.Eval(st, rootPlayer)
}
