//go:build js && wasm

// Command botwasm exposes a 1v1 ROOT game against the engine to the browser as
// WebAssembly. The game state lives in the browser, so the demo needs no server
// and can be hosted as a static site (GitHub Pages).
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
	humanFaction       = root.MC
	botFaction         = root.ED
	botSpec            = "mcts:full"
	botSims            = 200
	botRollout         = 0
	botSeed      int64 = 1
	gameSeed     int
)

func newGame(human string, seed, sims int) {
	h := root.Faction(human)
	if h != root.MC && h != root.ED {
		h = root.MC
	}
	b := root.ED
	if h == root.ED {
		b = root.MC
	}
	humanFaction, botFaction = h, b
	if sims > 0 {
		botSims = sims
	}
	current = root.NewGame([]root.Faction{root.MC, root.ED}, root.MC, uint64(seed))
	root.BeginSetup(current)
	botSeed = int64(seed) + 1
	gameSeed = seed
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

func makeBot() bot.Bot {
	defer func() { recover() }()
	return bot.Make(botSpec, botSeed, botSims, botRollout)
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
	snap["bot"] = string(botFaction)
	snap["humanActor"] = string(humanFaction) == string(current.Actor())
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
			human := "MC"
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
			newGame(human, seed, sims)
			return snapshotJSON()
		}),
		"apply": js.FuncOf(func(this js.Value, args []js.Value) any {
			if current == nil || len(args) < 1 {
				return withError("no game")
			}
			id := args[0].String()
			if err := current.Apply(root.Action{ID: id}); err != nil {
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
		// botStep plays exactly one engine action and returns it with the
		// resulting state, so the client can animate the bot's turn.
		"botStep": js.FuncOf(func(this js.Value, args []js.Value) any {
			if current == nil || len(current.Winner) > 0 || current.Actor() != botFaction {
				return `{"done":true}`
			}
			if len(current.LegalActions()) == 0 {
				return `{"done":true}`
			}
			mv := makeBot().Choose(current, botFaction)
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
		"setBot": js.FuncOf(func(this js.Value, args []js.Value) any {
			if len(args) > 0 {
				botSpec = args[0].String()
			}
			if len(args) > 1 {
				botSims = args[1].Int()
			}
			return fmt.Sprintf("%s/%d", botSpec, botSims)
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
