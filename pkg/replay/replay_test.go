package replay

import (
	"strings"
	"testing"

	"github.com/Thanhphan1147/root-mn/pkg/root"
)

// TestReplayRoundTrip plays a short bot-free game by always taking the first
// legal action, exports the RMN, and checks the replay reconstructs the same
// final position — exercising SYS roll/shuffle lines and multi-line actions.
func TestReplayRoundTrip(t *testing.T) {
	order := []root.Faction{root.MC, root.ED, root.WA, root.VB}
	seed := uint64(7)
	g := root.NewGame(order, order[0], seed)
	root.BeginSetup(g)
	for i := 0; i < 60 && len(g.Winner) == 0; i++ {
		acts := g.LegalActions()
		if len(acts) == 0 {
			break
		}
		if err := g.Apply(acts[0]); err != nil {
			t.Fatalf("apply: %v", err)
		}
	}

	var b strings.Builder
	b.WriteString("%RMN 3.0\n")
	b.WriteString("%Game test-7\n")
	b.WriteString("%Map autumn\n%Deck standard\n")
	for i, f := range order {
		b.WriteString("%Faction " + string(f) + " " + f.Kind() + " seat=" + itoa(i+1) + "\n")
	}
	b.WriteString("%First " + string(order[0]) + "\n")
	for _, line := range g.RMNLog {
		b.WriteString(line + "\n")
	}

	want, _ := root.Snapshot(g)["hash"].(string)
	res, err := Run(b.String())
	if err != nil {
		t.Fatal(err)
	}
	if res.Failure != "" {
		t.Fatalf("replay failed: %s", res.Failure)
	}
	last := res.Start
	if len(res.Events) > 0 {
		last = res.Events[len(res.Events)-1].State
	}
	if got, _ := last["hash"].(string); got != want {
		t.Fatalf("replayed hash %s != %s", got, want)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [4]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
