package main

import (
	"math/rand"
	"testing"

	"github.com/Thanhphan1147/root-mn/pkg/root"
)

func freshGame(seed uint64) *root.Game {
	g := root.NewGame([]root.Faction{root.MC, root.ED}, root.MC, seed)
	root.BeginSetup(g)
	rng := rand.New(rand.NewSource(int64(seed)))
	for i := 0; i < 200; i++ {
		acts := g.LegalActions()
		if len(acts) == 0 {
			break
		}
		_ = g.Apply(acts[rng.Intn(len(acts))])
	}
	return g
}

func BenchmarkLegalActions(b *testing.B) {
	g := freshGame(7)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.LegalActions()
	}
}

func BenchmarkClone(b *testing.B) {
	g := freshGame(7)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.Clone()
	}
}

func BenchmarkCloneForSearch(b *testing.B) {
	g := freshGame(7)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.CloneForSearch()
	}
}
