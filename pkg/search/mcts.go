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
//
// Nodes at depth <= PriorDepth are ordered and weighted by a one-ply eval
// (PUCT) so near the root the search tries the promising actions first; deeper
// nodes use plain UCT. Computing the eval prior costs one clone+apply+eval per
// action, so keeping PriorDepth small (0 = root only) keeps it cheap.
type MCTS struct {
	Eval         eval.Evaluator
	Rng          *rand.Rand
	Sims         int     // simulations per decision (split across worlds)
	RolloutDepth int     // random plies before evaluating
	C            float64 // exploration constant
	PriorTemp    float64 // softmax temperature for the eval prior (0 => 3)
	PriorDepth   int     // plies below the root that get an eval prior
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
	if m.PriorTemp <= 0 {
		m.PriorTemp = 3
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
			if c == nil {
				continue
			}
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
	state     *root.Game
	player    root.Faction
	action    root.Action // the action that produced this node
	actions   []root.Action
	depth     int
	priors    []float64 // parallel to actions; set when the node is expanded
	evalPrior bool      // true when priors come from the evaluator (PUCT)
	children  []*mnode  // parallel to actions; nil until that child is visited
	visits    int
	value     float64 // sum of evaluations from the root player's perspective
}

func (m MCTS) simulate(rootNode *mnode, rootPlayer root.Faction, r *rand.Rand) {
	n := rootNode
	path := []*mnode{n}
	for {
		if len(n.actions) == 0 || len(n.state.Winner) > 0 {
			break
		}
		if n.priors == nil {
			m.expand(n)
		}
		// Expansion step: open exactly one new node, then stop and score it.
		if created := m.unexpanded(n); created != nil {
			n = created
			path = append(path, n)
			break
		}
		// Otherwise descend through already-expanded children.
		sel := m.selectExisting(n, rootPlayer)
		if sel == nil {
			break
		}
		n = sel
		path = append(path, n)
	}
	v := m.rollout(n.state, rootPlayer, r)
	for _, p := range path {
		p.visits++
		p.value += v
	}
}

// unexpanded materialises the highest-prior action that has no child yet, or
// returns nil when every action has been expanded.
func (m MCTS) unexpanded(n *mnode) *mnode {
	bestI, bestP := -1, math.Inf(-1)
	for i := range n.actions {
		if n.children[i] != nil {
			continue
		}
		if n.priors[i] > bestP {
			bestP, bestI = n.priors[i], i
		}
	}
	if bestI < 0 {
		return nil
	}
	a := n.actions[bestI]
	child := n.state.CloneForSearch()
	if err := child.ApplyFast(a); err != nil {
		// Mark it so the same illegal action is not retried forever.
		n.children[bestI] = &mnode{action: a, depth: n.depth + 1}
		return nil
	}
	cn := &mnode{state: child, player: child.Actor(), actions: child.LegalActions(), action: a, depth: n.depth + 1}
	n.children[bestI] = cn
	return cn
}

// expand builds the prior for every action of n. Near the root it uses a
// one-ply eval (PUCT); deeper it uses a uniform prior (UCT) to avoid paying a
// clone+apply+eval per action at every node.
func (m MCTS) expand(n *mnode) {
	n.priors = make([]float64, len(n.actions))
	n.children = make([]*mnode, len(n.actions))
	if n.depth > m.PriorDepth {
		u := 1.0 / float64(len(n.actions))
		for i := range n.priors {
			n.priors[i] = u
		}
		return
	}
	vals := make([]float64, len(n.actions))
	for i, a := range n.actions {
		child := n.state.CloneForSearch()
		if err := child.ApplyFast(a); err != nil {
			vals[i] = math.Inf(-1)
			continue
		}
		// Score from the acting player's own view: this node's player chooses
		// among n.actions.
		vals[i] = m.Eval.Eval(child, n.player)
	}
	n.priors = softmax(vals, m.PriorTemp)
	n.evalPrior = true
}

func (m MCTS) selectExisting(n *mnode, rootPlayer root.Faction) *mnode {
	// Pick by PUCT (eval prior) or UCT (uniform).
	var best *mnode
	bestScore := math.Inf(-1)
	logN := math.Log(float64(n.visits) + 1)
	sqrtN := math.Sqrt(float64(n.visits) + 1)
	dir := 1.0
	if n.player != rootPlayer {
		dir = -1 // the opponent minimizes the root player's value
	}
	for i, c := range n.children {
		if c == nil || c.visits == 0 {
			continue
		}
		q := c.value / float64(c.visits)
		var u float64
		if n.evalPrior {
			u = m.C * n.priors[i] * sqrtN / (1 + float64(c.visits))
		} else {
			u = m.C * math.Sqrt(logN/float64(c.visits))
		}
		score := dir*q + u
		if score > bestScore {
			bestScore, best = score, c
		}
	}
	return best
}

// softmax converts one-ply values into a prior distribution. Non-finite values
// (illegal actions) get zero.
func softmax(vals []float64, temp float64) []float64 {
	out := make([]float64, len(vals))
	max := math.Inf(-1)
	for _, v := range vals {
		if math.IsInf(v, 0) {
			continue
		}
		if v > max {
			max = v
		}
	}
	if math.IsInf(max, -1) {
		return out
	}
	var sum float64
	for i, v := range vals {
		if math.IsInf(v, 0) {
			continue
		}
		e := math.Exp((v - max) / temp)
		out[i] = e
		sum += e
	}
	if sum == 0 {
		return out
	}
	for i := range out {
		out[i] /= sum
	}
	return out
}

// rollout scores a position: with no rollout depth it evaluates the frontier
// directly (the evaluator does not mutate the state, so no copy is needed);
// otherwise it plays random plies from a copy, then evaluates.
func (m MCTS) rollout(g *root.Game, rootPlayer root.Faction, r *rand.Rand) float64 {
	if m.RolloutDepth <= 0 {
		return m.Eval.Eval(g, rootPlayer)
	}
	st := g.CloneForSearch()
	for d := 0; d < m.RolloutDepth; d++ {
		if len(st.Winner) > 0 {
			break
		}
		acts := st.LegalActions()
		if len(acts) == 0 {
			break
		}
		if err := st.ApplyFast(acts[r.Intn(len(acts))]); err != nil {
			break
		}
	}
	return m.Eval.Eval(st, rootPlayer)
}
