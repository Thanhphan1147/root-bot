package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Thanhphan1147/root-mn/pkg/root"
)

func main() {
	text, _ := os.ReadFile(os.Args[1])
	sc := bufio.NewScanner(strings.NewReader(string(text)))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var lines []string
	var order []root.Faction
	var first root.Faction
	var seed uint64
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, "%") {
			f := strings.Fields(l)
			switch f[0] {
			case "%Game":
				s := f[1]
				if i := strings.LastIndex(s, "-"); i >= 0 {
					s = s[i+1:]
				}
				seed, _ = strconv.ParseUint(s, 10, 64)
			case "%Faction":
				order = append(order, root.Faction(f[1]))
			case "%First":
				first = root.Faction(f[1])
			}
			continue
		}
		lines = append(lines, l)
	}
	g := root.NewGame(order, first, seed)
	root.BeginSetup(g)
	for i, line := range lines {
		if strings.Contains(line, "coalition") {
			fmt.Printf("before line %d: %s\n", i+1, line)
			for _, f := range g.Order {
				p := g.Players[f]
				fmt.Printf("  %s VP=%d coalition=%q dominanceActive=%v\n", f, p.VP, p.Coalition, dominanceOf(g, f))
			}
		}
		if err := g.ApplyRMN(line); err != nil {
			fmt.Println("ERR", line, err)
			break
		}
	}
}

func dominanceOf(g *root.Game, f root.Faction) string {
	for id, owner := range g.DominanceActive {
		if owner == f {
			return id
		}
	}
	return ""
}
