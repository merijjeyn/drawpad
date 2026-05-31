<!--
Paste the section below into your project's AGENTS.md / CLAUDE.md /
.cursor/rules / system prompt. It tells your coding agent that the
`drawpad` CLI is available and when to reach for it.

Source: https://github.com/merijjeyn/drawpad
-->

## drawpad — collaborate with the user on a diagram

`drawpad` is a CLI on `$PATH` that opens a small Excalidraw window so
the human can sketch, red-line, or annotate, then returns their drawing
as JSON plus a PNG screenshot. Use it whenever a picture would land
faster than three rounds of prose.

**Good fits:**

- Showcasing a proposed architecture before writing code
- UI / UX wireframes you want the user to red-line
- Flows, sequences, or state machines for review
- Inviting the user to sketch *their* idea instead of describing it

**Skip it for:** trivial confirmations, anything that fits in one line of
text, or headless environments (CI, SSH-without-display).

### Invocation

```bash
drawpad \
  --prompt   "Does this match what you had in mind?" \
  --initial  scene.json \      # optional; omit for a blank canvas; "-" for stdin
  --output   result.json \     # or omit / "-" for stdout
  --screenshot-dir ./screenshots
```

Full flag list: `drawpad --help`.

### Result

The CLI exits with:

- **0** — user clicked **Send**. `result.json` has `scene`, `comment`,
  `screenshotPath`, `cancelled: false`. **Open the PNG at
  `screenshotPath`** to actually see what the user drew — the drawing is
  the answer. Treat `comment` as the user's primary natural-language reply.
- **2** — user cancelled or closed the window. `cancelled: true`. Don't
  pretend they confirmed; fall back to asking in text.
- **1** — config / I/O error; stderr explains.

### Authoring the initial scene

You don't have to hand-write the Excalidraw JSON. Write a tiny throwaway
Go program in `/tmp/` that emits the `{type, version, elements,
appState}` envelope and pipe it in:

```bash
go run /tmp/my-sketch/main.go | drawpad --initial - --output result.json
```

Each element is a plain map with at minimum `id`, `type` (`rectangle`,
`ellipse`, `diamond`, `text`, `arrow`, `line`, ...), and geometry
(`x`, `y`, `width`, `height`). Unknown fields round-trip untouched.
