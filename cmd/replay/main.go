// Command replay drives the engine from an RMN log so a game can be inspected
// at any point. It matches each logged line to the legal action whose own RMN
// line is identical, which works because the engine produced the log.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Thanhphan1147/root-mn/pkg/root"
)

func main() {
	file := flag.String("file", "", "RMN file to replay")
	seedFlag := flag.Int64("seed", 0, "override seed (else read from %Game demo-<n>)")
	stop := flag.Int("stop", 0, "stop after this RMN seq (0 = end)")
	dump := flag.Bool("dump", true, "dump state + legal actions at the stop point")
	leader := flag.String("leader", "", "force the ED setup leader (disambiguates the log)")
	flag.Parse()
	forcedLeader = *leader
	if *file == "" {
		fmt.Fprintln(os.Stderr, "need -file")
		os.Exit(2)
	}

	lines, seed, err := readLog(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *seedFlag != 0 {
		seed = *seedFlag
	}
	fmt.Printf("seed=%d events=%d\n", seed, len(lines))

	g := root.NewGame([]root.Faction{root.MC, root.ED}, root.MC, uint64(seed))
	root.BeginSetup(g)

	ambiguous := 0
	for i, line := range lines {
		seq := lineSeq(line)
		if *stop > 0 && seq > *stop {
			break
		}
		a, matches, ok := matchLine(g, line)
		if !ok {
			fmt.Printf("\nGAP at event %d: %q\n", i, line)
			fmt.Println("legal actions and their RMN lines:")
			for _, la := range g.LegalActions() {
				fmt.Printf("  %-60s | %s\n", la.ID, rmnOf(g, la))
			}
			return
		}
		if matches > 1 {
			ambiguous++
			fmt.Printf("AMBIGUOUS event %d (%d matches): %s\n", i, matches, line)
		}
		if err := g.Apply(a); err != nil {
			fmt.Printf("apply failed at %q: %v\n", line, err)
			return
		}
	}

	if ambiguous == 0 {
		fmt.Printf("\nreplayed all %d events with no ambiguity\n", len(lines))
	} else {
		fmt.Printf("\nreplayed all %d events; %d ambiguous lines\n", len(lines), ambiguous)
	}
	fmt.Printf("round %d.%s, actor=%s, MC vp=%d ED vp=%d, winner=%v\n",
		g.Round, g.Phase, g.Actor(), g.Players[root.MC].VP, g.Players[root.ED].VP, g.Winner)
	if *dump {
		dumpState(g)
	}
}

func readLog(path string) ([]string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	var lines []string
	var seed int64
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		t := sc.Text()
		if strings.HasPrefix(t, "%Game demo-") {
			if n, err := strconv.ParseInt(strings.TrimPrefix(t, "%Game demo-"), 10, 64); err == nil {
				seed = n
			}
		}
		if strings.HasPrefix(t, "%") || strings.TrimSpace(t) == "" {
			continue
		}
		lines = append(lines, t)
	}
	return lines, seed, sc.Err()
}

func lineSeq(line string) int {
	f := strings.Fields(line)
	if len(f) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(f[0])
	return n
}

// matchLine returns the legal action whose RMN output equals line, plus how many
// legal actions produced that exact line (more than one means the log is
// under-specified).
func matchLine(g *root.Game, line string) (root.Action, int, bool) {
	base := len(g.RMNLog)
	var found root.Action
	n := 0
	for _, a := range g.LegalActions() {
		if forcedLeader != "" && a.Kind == "setup-ed-leader" && a.Leader != forcedLeader {
			continue
		}
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
	return found, n, n > 0
}

var forcedLeader string

func rmnOf(g *root.Game, a root.Action) string {
	base := len(g.RMNLog)
	c := g.Clone()
	if err := c.Apply(a); err != nil {
		return "<" + err.Error() + ">"
	}
	if len(c.RMNLog) > base {
		return c.RMNLog[base]
	}
	return ""
}

func dumpState(g *root.Game) {
	mc, ed := g.Players[root.MC], g.Players[root.ED]
	fmt.Printf("MC: wood-board=%d supply=%d sawmills=%d workshops=%d recruiters=%d\n",
		boardWood(g), mc.WoodSupply, mc.Sawmills, mc.Workshops, mc.Recruiters)
	fmt.Printf("ED: leader=%q roosts=%d retired=%v decree=%v queue=%v\n",
		ed.Leader, countType(g, root.ED, "roost"), ed.RetiredLeaders, ed.Decree, g.DecreeQueue)
	fmt.Println("legal MC build actions:")
	for _, a := range g.LegalActions() {
		if a.Kind == "mc-build" {
			fmt.Printf("  %s\n", a.Label)
		}
	}
	fmt.Println("C9 detail:")
	cl := g.Clearings["C9"]
	if cl != nil {
		fmt.Printf("  suit=%s slots=%d free=%d warriors=%v buildings=%v wood=%d ruled(MC)=%v\n",
			cl.Suit, cl.Slots, cl.FreeSlots(), cl.Warriors, cl.Buildings, cl.Wood, g.Rules(root.MC, "C9"))
	}
}

func boardWood(g *root.Game) int {
	n := 0
	for _, c := range g.Clearings {
		n += c.Wood
	}
	return n
}

func countType(g *root.Game, f root.Faction, typ string) int {
	n := 0
	for _, c := range g.Clearings {
		for _, b := range c.Buildings {
			if b.Owner == f && b.Type == typ {
				n++
			}
		}
	}
	return n
}
