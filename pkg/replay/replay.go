// Package replay reconstructs a game from an RMN log so it can be inspected or
// animated. It matches each logged line to the unique legal action that produced
// it, which works because the engine generated the log in the first place.
package replay

import (
	"bufio"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Thanhphan1147/root-mn/pkg/root"
)

// Event is one replayed action and the board state after it.
type Event struct {
	Line   string         `json:"line"`
	Action root.Action    `json:"action"`
	State  map[string]any `json:"state"`
}

// Result is a fully replayed log.
type Result struct {
	Seed    int64                    `json:"seed"`
	Order   []root.Faction           `json:"order"`
	Winner  []root.Faction           `json:"winner"`
	Cards   map[string]root.CardInfo `json:"cards"`
	Start   map[string]any           `json:"start"`
	Events  []Event                  `json:"events"`
	Failure string                   `json:"failure,omitempty"` // set when a line could not be replayed
}

// Parse extracts the header (seed, factions, first player) and body lines.
func Parse(text string) (seed int64, order []root.Faction, first root.Faction, body []string, err error) {
	type seat struct {
		f root.Faction
		n int
	}
	var seats []seat
	var mapped string
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "%") {
			body = append(body, line)
			continue
		}
		f := strings.Fields(line)
		switch f[0] {
		case "%Game":
			if len(f) >= 2 {
				s := strings.TrimPrefix(f[1], "demo-")
				if n, e := strconv.ParseInt(s, 10, 64); e == nil {
					seed = n
				}
			}
		case "%Map":
			if len(f) >= 2 {
				mapped = f[1]
			}
		case "%First":
			if len(f) >= 2 {
				first = root.Faction(f[1])
			}
		case "%Faction":
			if len(f) >= 2 {
				s := seat{f: root.Faction(f[1])}
				for _, x := range f[2:] {
					if strings.HasPrefix(x, "seat=") {
						s.n, _ = strconv.Atoi(strings.TrimPrefix(x, "seat="))
					}
				}
				seats = append(seats, s)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return 0, nil, "", nil, err
	}
	if mapped != "" && mapped != "autumn" {
		return 0, nil, "", nil, fmt.Errorf("map %q is not supported (only autumn)", mapped)
	}
	if len(seats) == 0 {
		return 0, nil, "", nil, fmt.Errorf("no %%Faction lines found")
	}
	sort.Slice(seats, func(i, j int) bool { return seats[i].n < seats[j].n })
	for _, s := range seats {
		order = append(order, s.f)
	}
	if first == "" {
		first = order[0]
	}
	return seed, order, first, body, nil
}

// match returns the legal action whose RMN line equals line, and how many legal
// actions produced that exact line (more than one means the log is ambiguous).
// Actions are visited in a stable (ID-sorted) order so matching does not depend
// on Go's map iteration order.
func match(g *root.Game, line string) (root.Action, int) {
	base := len(g.RMNLog)
	legal := g.LegalActions()
	sort.Slice(legal, func(i, j int) bool { return legal[i].ID < legal[j].ID })
	var found root.Action
	n := 0
	for _, a := range legal {
		c := g.Clone()
		if err := c.Apply(a); err != nil {
			continue
		}
		if len(c.RMNLog) == base+1 && c.RMNLog[base] == line {
			n++
			if n == 1 {
				found = a
			}
		}
	}
	return found, n
}

// leanState is a snapshot of a deep copy of the game: root.Snapshot returns the
// live Clearings/Players by reference, so without the clone every step would
// encode the final board. It drops the RMN (which grows every step and would
// make the payload quadratic) and `legal`/`cards` (not needed to draw the
// board), and trims the human log to its tail.
func leanState(g *root.Game) map[string]any {
	c := g.CloneForSearch() // deep copy; drops logs
	s := root.Snapshot(c)
	delete(s, "rmn")
	delete(s, "legal")
	delete(s, "cards")
	l := g.Log
	if len(l) > 40 {
		l = l[len(l)-40:]
	}
	s["log"] = l
	return s
}

// Run replays a full log.
func Run(text string) (*Result, error) {
	seed, order, first, body, err := Parse(text)
	if err != nil {
		return nil, err
	}
	g := root.NewGame(order, first, uint64(seed))
	root.BeginSetup(g)

	res := &Result{Seed: seed, Order: order, Cards: root.CardInfoMap(), Start: leanState(g)}
	for i, line := range body {
		a, n := match(g, line)
		if n == 0 {
			res.Failure = fmt.Sprintf("event %d could not be replayed: %s", i+1, line)
			res.Winner = g.Winner
			return res, nil
		}
		if err := g.Apply(a); err != nil {
			res.Failure = fmt.Sprintf("event %d apply failed: %v", i+1, err)
			res.Winner = g.Winner
			return res, nil
		}
		res.Events = append(res.Events, Event{Line: line, Action: a, State: leanState(g)})
	}
	res.Winner = g.Winner
	return res, nil
}
