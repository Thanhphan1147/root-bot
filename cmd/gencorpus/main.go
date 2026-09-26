// Command gencorpus plays games with the shipped bots and writes their RMN logs
// (plus the final state hash) as a validation corpus for the engine.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Thanhphan1147/root-bot/pkg/bot"
	"github.com/Thanhphan1147/root-mn/pkg/root"
)

type cfg struct {
	name  string
	order []root.Faction
	specs map[root.Faction]string
}

func main() {
	n := flag.Int("n", 100, "games")
	out := flag.String("out", "", "output directory")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "need -out")
		os.Exit(2)
	}
	cfgs := []cfg{
		{"2p", []root.Faction{root.MC, root.ED}, map[root.Faction]string{root.MC: "greedy:material", root.ED: "greedy:eyrie"}},
		{"4p", []root.Faction{root.MC, root.ED, root.WA, root.VB}, map[root.Faction]string{root.MC: "greedy:material", root.ED: "greedy:eyrie", root.WA: "greedy:wa", root.VB: "greedy:vb"}},
	}

	written, skipped := 0, 0
	for i := 0; i < *n; i++ {
		c := cfgs[i%len(cfgs)]
		seed := 1000 + uint64(i)
		g := play(c, seed)
		if len(g.Winner) == 0 {
			skipped++
			continue
		}
		name := fmt.Sprintf("%s-%04d", c.name, i)
		if err := os.WriteFile(filepath.Join(*out, name+".rmn"), []byte(export(c, g, seed)), 0o644); err != nil {
			panic(err)
		}
		hash, _ := root.Snapshot(g)["hash"].(string)
		os.WriteFile(filepath.Join(*out, name+".hash"), []byte(hash+"\n"), 0o644)
		written++
	}
	fmt.Printf("wrote %d games (%d unfinished skipped)\n", written, skipped)
}

func play(c cfg, seed uint64) *root.Game {
	g := root.NewGame(c.order, c.order[0], seed)
	root.BeginSetup(g)
	for i := 0; i < 40000; i++ {
		if len(g.Winner) > 0 || len(g.LegalActions()) == 0 {
			break
		}
		f := g.Actor()
		b := bot.Make(c.specs[f], int64(seed)+int64(len(f)), 0, 0)
		mv := b.Choose(g, f)
		if mv.ID == "" {
			break
		}
		if err := g.Apply(mv); err != nil {
			break
		}
	}
	return g
}

func export(c cfg, g *root.Game, seed uint64) string {
	s := "%RMN 3.0\n"
	s += fmt.Sprintf("%%Game corpus-%d\n", seed)
	s += "%Map autumn\n%Deck standard\n"
	for i, f := range c.order {
		s += fmt.Sprintf("%%Faction %s %s seat=%d\n", f, f.Kind(), i+1)
	}
	s += fmt.Sprintf("%%First %s\n", g.First)
	for _, line := range g.RMNLog {
		s += line + "\n"
	}
	return s
}
