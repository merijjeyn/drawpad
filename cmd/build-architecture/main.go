// build-architecture writes an Excalidraw scene describing drawpad's
// own architecture to stdout. Pipe it to `drawpad --initial -` to open it.
package main

import (
	"encoding/json"
	"os"

	"github.com/merijjeyn/drawpad/internal/diagram"
)

func main() {
	s := diagram.NewScene()

	// Layout: vertical stack of layers (game-engine style), bottom = lowest,
	// each layer is a rectangle with a centered label.
	type layer struct {
		title  string
		fill   string
		detail string
	}
	layers := []layer{
		{"L6  cmd/drawpad  (CLI entry)", "#fde68a", "flag parsing • result writer"},
		{"L5  internal/app  (orchestrator)", "#fcd34d", "wires session + server + browser • blocks on Run()"},
		{"L4  internal/browser  (Chrome --app launcher)", "#fbbf24", "exec chrome • ctx-driven kill"},
		{"L3  web/  (Excalidraw frontend)", "#a7f3d0", "React 17 + Excalidraw 0.17 UMD • exportToBlob"},
		{"L2  internal/server  (HTTP + embedded assets)", "#86efac", "/api/init • /api/submit • /api/cancel"},
		{"L1  internal/session  (one-shot rendezvous)", "#a5f3fc", "Complete-once gate • Wait(ctx)"},
		{"L0  internal/diagram  (domain model)", "#bfdbfe", "Scene • Element • Result • Submission"},
	}

	const (
		startX  = 80.0
		startY  = 60.0
		boxW    = 520.0
		boxH    = 64.0
		gap     = 16.0
		detailW = 360.0
	)

	for i, l := range layers {
		y := startY + float64(i)*(boxH+gap)
		box := diagram.Rectangle(startX, y, boxW, boxH)
		box["backgroundColor"] = l.fill
		box["fillStyle"] = "solid"
		label := diagram.Text(0, 0, l.title)
		label["fontSize"] = 16.0
		diagram.BindText(box, label)
		s.Elements = append(s.Elements, box, label)

		// Right-side detail caption.
		detail := diagram.Text(startX+boxW+24, y+boxH/2-12, l.detail)
		detail["fontSize"] = 13.0
		detail["strokeColor"] = "#4b5563"
		s.Elements = append(s.Elements, detail)
		_ = detailW
	}

	// Side column: external actors.
	agent := diagram.Rectangle(startX+boxW+detailW+80, startY, 200, 80)
	agent["backgroundColor"] = "#f9a8d4"
	agent["fillStyle"] = "solid"
	agentText := diagram.Text(0, 0, "AI agent\n(runs `drawpad`)")
	agentText["fontSize"] = 16.0
	diagram.BindText(agent, agentText)
	s.Elements = append(s.Elements, agent, agentText)

	user := diagram.Rectangle(startX+boxW+detailW+80, startY+220, 200, 80)
	user["backgroundColor"] = "#c4b5fd"
	user["fillStyle"] = "solid"
	userText := diagram.Text(0, 0, "Human\n(edits in Chrome --app)")
	userText["fontSize"] = 16.0
	diagram.BindText(user, userText)
	s.Elements = append(s.Elements, user, userText)

	// Arrows showing the request/response flow.
	cliBox := s.Elements[0] // L6 is the first appended pair → index 0
	// Find the actual L3 (web) box → index 6 (each layer adds box + label)
	webBox := s.Elements[6]

	a1 := diagram.Arrow(0, 0, 0, 0)
	a1["strokeColor"] = "#be185d"
	diagram.BindArrow(a1, agent, cliBox)
	label1 := diagram.Text(startX+boxW+detailW+90, startY+90, "stdin / flags →")
	label1["fontSize"] = 12.0
	label1["strokeColor"] = "#be185d"

	a2 := diagram.Arrow(0, 0, 0, 0)
	a2["strokeColor"] = "#5b21b6"
	diagram.BindArrow(a2, webBox, user)
	label2 := diagram.Text(startX+boxW+detailW+90, startY+260, "Excalidraw UI")
	label2["fontSize"] = 12.0
	label2["strokeColor"] = "#5b21b6"

	a3 := diagram.Arrow(0, 0, 0, 0)
	a3["strokeColor"] = "#0f766e"
	diagram.BindArrow(a3, cliBox, agent)
	label3 := diagram.Text(startX+boxW+detailW+90, startY+30, "← result.json + PNG path")
	label3["fontSize"] = 12.0
	label3["strokeColor"] = "#0f766e"

	s.Elements = append(s.Elements, a1, label1, a2, label2, a3, label3)

	// Title at the top.
	title := diagram.Text(startX, 10, "drawpad — architecture (built one-shot)")
	title["fontSize"] = 22.0
	title["strokeColor"] = "#111827"
	s.Elements = append(s.Elements, title)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		panic(err)
	}
}
