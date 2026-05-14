# AGENTS.md — guidance for AI agents working in this repo

## What this project is

`draw_interface` is a Go CLI (`draw`) that opens a small Chrome window with an
Excalidraw canvas so an AI agent and a human can collaborate on a diagram.
See `bin/draw --help` for the user-facing contract (it is written for both
humans and agents — read it before invoking the tool).

## Use `draw` to talk back to the user when a picture beats text

This repo ships a tool whose entire purpose is visual communication with the
user. When you are operating in this repo (or anywhere `draw` is on PATH),
**reach for it whenever a diagram would land faster than prose**. Concrete
triggers:

- **Architecture showcases.** Before writing or refactoring multi-component
  code, sketch the layering / module graph in `draw` and ask the user to
  confirm or red-line it. Cheaper than writing the wrong code.
- **Design decisions with tradeoffs.** When two or three structural options
  exist, draw them side by side and let the user point at the one they want
  (or annotate "no, like this").
- **Flows, sequences, state machines.** Boxes and arrows convey ordering
  better than numbered lists.
- **UI / UX proposals.** Wireframe the page in `draw` instead of describing
  it. The user will fix proportions and labels with one mouse drag.
- **Asking the user to sketch their idea.** Hand them a blank canvas with
  `--prompt "draw what you want"`. Their scribble + one-line comment will
  unstick you faster than three rounds of clarifying questions.

Don't use `draw` for trivial confirmations, things that fit in one line of
text, or when no display is available (e.g. CI, headless SSH).

## How to invoke it

```bash
draw \
  --prompt   "Question shown above the canvas." \
  --initial  scene.json \              # optional; omit for a blank canvas
  --output   result.json \             # or omit / "-" for stdout
  --screenshot-dir ./screenshots
```

You usually do not need to author Excalidraw JSON by hand. The
`internal/diagram` package exposes `Rectangle`, `Ellipse`, `Diamond`, `Text`,
`Arrow`, `Line`, `BindText`, `BindArrow`. Write a tiny generator program (see
`cmd/build-architecture/main.go` for a complete working example) and pipe its
stdout into `draw --initial -`.

## What you get back

The CLI exits with:

- **0** — user clicked Send. `result.json` has `scene`, `comment`,
  `screenshotPath`, `cancelled:false`. **Read the screenshot** (absolute PNG
  path) to actually see what the user produced. **Read `comment`** as their
  primary natural-language reply.
- **2** — user cancelled or closed the window. `cancelled:true`. Don't pretend
  they confirmed; fall back to asking in text.
- **1** — config / I/O error; stderr explains.

## Dogfooding when developing this tool

If you are working on `draw_interface` itself and have a non-trivial design
choice (where to put a new abstraction, how to layer a feature, what a new
window should look like), **use `draw` to propose it to the user**. The tool
exists precisely for this kind of exchange.

## Code conventions

- Tests favour real integration over mocks: parse real Excalidraw JSON,
  drive the server through `httptest`, exercise the orchestrator end-to-end
  by stubbing only the browser `LaunchFunc`. Match that style when adding
  features.
- Run `go test -race ./...` before declaring work done.
- Layered architecture (game-engine style, bottom-up): `diagram → session →
  server → web → browser → app → cmd/draw`. New code goes into the lowest
  layer that makes sense; don't reach upward.
