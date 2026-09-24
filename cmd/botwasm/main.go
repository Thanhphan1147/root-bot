//go:build js && wasm

// Command botwasm exposes a ROOT game against the engine to the browser as
// WebAssembly. The game state lives in the browser, so the demo needs no server
// and can be hosted as a static site (GitHub Pages). It runs 1v1 (Marquise vs
// Eyrie) or the full four-player base game.
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"syscall/js"

	"github.com/Thanhphan1147/root-bot/pkg/bot"
	"github.com/Thanhphan1147/root-bot/pkg/replay"
	"github.com/Thanhphan1147/root-mn/pkg/root"
)

var (
	current      *root.Game
	humanFaction = root.MC
	cpuFactions  = map[root.Faction]bool{}
	botSpecs     = map[root.Faction]string{
		root.MC: "mcts:material",
		root.ED: "mcts:eyrie",
		root.WA: "greedy:material",
		root.VB: "greedy:material",
	}
	botSims          = 200
	botSeed    int64 = 1
	gameSeed   int
	history    []*root.Game
	maxHistory = 240
)

func firstSentinel(order []root.Faction) root.Faction {
	if len(order) > 0 {
		return order[0]
	}
	return root.MC
}

func inOrder(order []root.Faction, f root.Faction) bool {
	for _, x := range order {
		if x == f {
			return true
		}
	}
	return false
}

// newGame starts a fresh game. mode is "1v1" (MC vs ED) or "4p" (all four
// base-game factions); human is the faction the player controls.
func newGame(human string, seed, sims int, mode string) {
	var order []root.Faction
	if mode == "4p" {
		order = []root.Faction{root.MC, root.ED, root.WA, root.VB}
		// Full-game testing runs a 1-ply greedy bot for every faction.
		botSpecs[root.MC] = "greedy:material"
		botSpecs[root.ED] = "greedy:eyrie"
		botSpecs[root.WA] = "greedy:material"
		botSpecs[root.VB] = "greedy:material"
	} else {
		order = []root.Faction{root.MC, root.ED}
		botSpecs[root.MC] = "mcts:material"
		botSpecs[root.ED] = "mcts:eyrie"
	}

	h := root.Faction(human)
	if !inOrder(order, h) {
		h = firstSentinel(order)
	}
	humanFaction = h
	cpuFactions = map[root.Faction]bool{}
	for _, f := range order {
		if f != h {
			cpuFactions[f] = true
		}
	}
	if sims > 0 {
		botSims = sims
	}
	current = root.NewGame(order, firstSentinel(order), uint64(seed))
	root.BeginSetup(current)
	botSeed = int64(seed) + 1
	gameSeed = seed
	history = nil
}

func pushHistory() {
	if current == nil {
		return
	}
	history = append(history, current.Clone())
	if len(history) > maxHistory {
		history = history[1:]
	}
}

// exportRMN returns the full, unredacted RMN for the current game, including
// every hidden draw and deal, for replay and debugging.
func exportRMN() string {
	if current == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("%RMN 3.0\n")
	fmt.Fprintf(&b, "%%Game demo-%d\n", gameSeed)
	b.WriteString("%Map autumn\n%Deck standard\n")
	for i, f := range current.Order {
		fmt.Fprintf(&b, "%%Faction %s %s seat=%d\n", f, f.Kind(), i+1)
	}
	if current.First != "" {
		fmt.Fprintf(&b, "%%First %s\n", current.First)
	}
	for _, line := range current.RMNLog {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func makeBotFor(f root.Faction) (b bot.Bot) {
	defer func() { recover() }()
	spec := botSpecs[f]
	if spec == "" {
		spec = "greedy:material"
	}
	return bot.Make(spec, botSeed, botSims, 0)
}

// snapshotJSON returns the client view. Redaction (hiding the engine's hand,
// supporters, and any hidden draws/deals in the log and RMN) is the engine's
// shared policy, so the demo hides exactly what a real opponent would.
func snapshotJSON() string {
	if current == nil {
		return "{}"
	}
	snap := root.Redact(current, string(humanFaction))
	snap["you"] = string(humanFaction)
	snap["humanActor"] = string(humanFaction) == string(current.Actor())
	snap["canUndo"] = len(history) > 0
	b, err := json.Marshal(snap)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func withError(msg string) string {
	b, _ := json.Marshal(map[string]any{"error": msg})
	return string(b)
}

func main() {
	api := map[string]any{
		"newGame": js.FuncOf(func(this js.Value, args []js.Value) any {
			human, mode := "MC", "1v1"
			seed, sims := 1, 0
			if len(args) > 0 {
				human = args[0].String()
			}
			if len(args) > 1 {
				seed = args[1].Int()
			}
			if len(args) > 2 {
				sims = args[2].Int()
			}
			if len(args) > 3 {
				mode = args[3].String()
			}
			newGame(human, seed, sims, mode)
			return snapshotJSON()
		}),
		"apply": js.FuncOf(func(this js.Value, args []js.Value) any {
			if current == nil || len(args) < 1 {
				return withError("no game")
			}
			id := args[0].String()
			pushHistory()
			if err := current.Apply(root.Action{ID: id}); err != nil {
				history = history[:len(history)-1] // discard the failed checkpoint
				out := map[string]any{"error": err.Error()}
				var snap map[string]any
				_ = json.Unmarshal([]byte(snapshotJSON()), &snap)
				for k, v := range snap {
					out[k] = v
				}
				b, _ := json.Marshal(out)
				return string(b)
			}
			return snapshotJSON()
		}),
		// undo restores the state before the player's last action (and any CPU
		// replies that followed it).
		"undo": js.FuncOf(func(this js.Value, args []js.Value) any {
			if current == nil {
				return withError("no game")
			}
			if len(history) == 0 {
				return withError("nothing to undo")
			}
			current = history[len(history)-1]
			history = history[:len(history)-1]
			return snapshotJSON()
		}),
		// botStep plays exactly one engine action and returns it with the
		// resulting state, so the client can animate the CPU's turn.
		"botStep": js.FuncOf(func(this js.Value, args []js.Value) any {
			if current == nil || len(current.Winner) > 0 {
				return `{"done":true}`
			}
			actor := current.Actor()
			if actor == humanFaction || !cpuFactions[actor] {
				return `{"done":true}`
			}
			if len(current.LegalActions()) == 0 {
				return `{"done":true}`
			}
			mv := makeBotFor(actor).Choose(current, actor)
			if mv.ID == "" {
				return `{"done":true}`
			}
			if err := current.Apply(mv); err != nil {
				return withError(err.Error())
			}
			ab, _ := json.Marshal(mv)
			var act, st any
			_ = json.Unmarshal(ab, &act)
			_ = json.Unmarshal([]byte(snapshotJSON()), &st)
			out, _ := json.Marshal(map[string]any{"action": act, "state": st})
			return string(out)
		}),
		// setBots takes a JSON object of faction -> spec (e.g. {"WA":"greedy:x"}).
		"setBots": js.FuncOf(func(this js.Value, args []js.Value) any {
			if len(args) > 0 {
				var specs map[string]string
				if err := json.Unmarshal([]byte(args[0].String()), &specs); err == nil {
					for k, v := range specs {
						botSpecs[root.Faction(k)] = v
					}
				}
			}
			if len(args) > 1 && args[1].Int() > 0 {
				botSims = args[1].Int()
			}
			return "ok"
		}),
		"snapshot": js.FuncOf(func(this js.Value, args []js.Value) any {
			return snapshotJSON()
		}),
		"export": js.FuncOf(func(this js.Value, args []js.Value) any {
			return exportRMN()
		}),
		// replay reconstructs a whole game from an RMN log: it returns the start
		// state plus one state (and action) per event, for a replay viewer.
		"replay": js.FuncOf(func(this js.Value, args []js.Value) any {
			if len(args) < 1 {
				return withError("no log")
			}
			res, err := replay.Run(args[0].String())
			if err != nil {
				return withError(err.Error())
			}
			b, err := json.Marshal(res)
			if err != nil {
				return withError(err.Error())
			}
			return string(b)
		}),
	}
	js.Global().Set("RootBot", js.ValueOf(api))
	select {}
}
