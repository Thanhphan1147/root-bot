// Package replay reconstructs a game from an RMN log so it can be inspected or
// animated. It replays each logged line through root.Game.ApplyRMN — the same
// authoritative path the corpus uses — so chance outcomes (SYS roll/shuffle),
// multi-line actions and notation changes are handled exactly as during play.
// It separately matches each action line to its legal action so the viewer can
// animate it.
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
				s := f[1]
				if i := strings.LastIndex(s, "-"); i >= 0 {
					s = s[i+1:]
				}
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
// An action may emit SYS lines (a dice roll or a recycle shuffle) before its own
// line, so we compare the LAST line it emits. During replay the SYS roll/recycle
// lines are consumed rather than appended to RMNLog, so sequence numbers drift;
// we therefore compare lines with their sequence prefix removed. Actions are
// visited in a stable (ID-sorted) order so matching does not depend on Go's map
// iteration order.
func match(g *root.Game, line string) (root.Action, int) {
	base := len(g.RMNLog)
	key := lineKey(line)
	legal := g.LegalActions()
	sort.Slice(legal, func(i, j int) bool { return legal[i].ID < legal[j].ID })
	var found root.Action
	n := 0
	for _, a := range legal {
		c := g.Clone()
		if err := c.Apply(a); err != nil {
			continue
		}
		if len(c.RMNLog) > base && lineKey(c.RMNLog[len(c.RMNLog)-1]) == key {
			n++
			if n == 1 {
				found = a
			}
		}
	}
	return found, n
}

// lineKey drops the leading sequence number from an RMN line so two lines can be
// compared even when a replay's sequence numbering has drifted.
func lineKey(line string) string {
	if i := strings.IndexByte(line, ' '); i >= 0 {
		return line[i+1:]
	}
	return line
}

// isSystemLine reports whether a logged line is a SYS event (assign-ruins,
// shuffle, roll) rather than a player action. SYS lines are applied to the game
// but are not replay steps of their own.
func isSystemLine(line string) bool {
	f := strings.Fields(line)
	return len(f) >= 4 && f[2] == "SYS"
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
		if isSystemLine(line) {
			// Apply chance/setup events (SYS roll/shuffle/assign-ruins) so the
			// state matches; they are not animation steps.
			if err := g.ApplyRMN(line); err != nil {
				res.Failure = fmt.Sprintf("event %d apply failed: %v", i+1, err)
				res.Winner = g.Winner
				return res, nil
			}
			continue
		}
		a, n := match(g, line)
		if n == 0 {
			res.Failure = fmt.Sprintf("event %d could not be replayed: %s", i+1, line)
			res.Winner = g.Winner
			return res, nil
		}
		if err := g.ApplyRMN(line); err != nil {
			res.Failure = fmt.Sprintf("event %d apply failed: %v", i+1, err)
			res.Winner = g.Winner
			return res, nil
		}
		res.Events = append(res.Events, Event{Line: line, Action: a, State: leanState(g)})
	}
	res.Winner = g.Winner
	return res, nil
}
