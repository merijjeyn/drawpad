# drawpad

A tiny CLI that lets an AI coding agent and a human collaborate on an
Excalidraw diagram. The agent calls `drawpad`, a Chrome window pops open
with a canvas, the human edits / scribbles / types a comment, hits
**Send**, and the agent gets back the final scene plus a PNG screenshot.

A picture beats three rounds of clarifying questions.

## Demo

<video src="https://github.com/merijjeyn/drawpad/raw/main/docs/demo.mp4" controls muted width="720">
  Your browser doesn't support HTML video.
  <a href="https://github.com/merijjeyn/drawpad/raw/main/docs/demo.mp4">Download the demo</a> instead.
</video>

## Install

### Recommended: one-line curl install (macOS / Linux)

```bash
curl -sSL https://raw.githubusercontent.com/merijjeyn/drawpad/main/scripts/install.sh | bash
```

The script auto-detects your OS / arch, downloads the matching prebuilt
binary from the [latest release](https://github.com/merijjeyn/drawpad/releases/latest),
verifies its SHA-256 checksum, drops it in `~/.local/bin`, and adds that
directory to your `$PATH` (in `~/.zshrc` / `~/.bashrc` / fish config) if
it isn't already. No Go toolchain required.

Knobs (all optional):

```bash
# Pin a version instead of "latest"
DRAWPAD_VERSION=v0.1.0 curl -sSL .../install.sh | bash

# Install somewhere else
DRAWPAD_INSTALL_DIR=/usr/local/bin curl -sSL .../install.sh | bash

# Don't touch shell rc files (just print the line you should add)
DRAWPAD_NO_MODIFY_PATH=1 curl -sSL .../install.sh | bash
```

### Alternative: `go install` (if you already have Go 1.23+)

```bash
go install github.com/merijjeyn/drawpad/cmd/drawpad@latest
```

This drops a `drawpad` binary into `$(go env GOBIN)` (or `$(go env
GOPATH)/bin`). You must make sure that directory is on your `PATH`
yourself — `go install` does **not** modify your shell rc. The standard
one-liner:

```bash
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc && source ~/.zshrc
```

### Manual download

Prebuilt binaries (including Windows) are attached to every release on
the [Releases page](https://github.com/merijjeyn/drawpad/releases).
Extract the archive and put `drawpad` somewhere on your `PATH`.

### Requirements (all install methods)

- **Google Chrome** installed locally — `drawpad` launches it in `--app`
  mode to host the canvas.
- **Internet access on first run** — the canvas pulls Excalidraw's UMD
  bundle from `unpkg.com`. On a fully offline machine the canvas won't
  render.

## Try it without an agent

Confirm the install works by handing yourself a blank canvas:

```bash
drawpad --prompt "Try drawing something, then hit Send."
```

Chrome should pop open. Draw a rectangle, type a comment, hit **Send**.
The CLI prints a JSON blob to stdout with your scene and a path to a PNG
of your drawing.

## Hook it up to your coding agent

This is the whole point. Once `drawpad` is on `$PATH`, your agent just
needs to *know it exists*. Tell it by pasting the snippet below into
wherever your agent reads project / system instructions:

| Agent              | Where to paste                                      |
| ------------------ | --------------------------------------------------- |
| **Amp**            | `AGENTS.md` at the repo root (auto-discovered)      |
| **Claude Code**    | `CLAUDE.md` at the repo root, or `~/.claude/CLAUDE.md` for global |
| **Cursor**         | `.cursor/rules/drawpad.mdc`                         |
| **OpenAI Codex CLI** | `AGENTS.md` (same convention as Amp)              |
| **Aider**          | `CONVENTIONS.md`, or pass with `--read`             |
| **Anything else**  | the agent's system / custom-instructions field      |

### The snippet

Grab the latest copy with:

```bash
curl -sSL https://raw.githubusercontent.com/merijjeyn/drawpad/main/docs/agent-snippet.md
```

Or copy from [docs/agent-snippet.md](./docs/agent-snippet.md). It's
deliberately short (~25 lines) so it fits in any agent's instruction
budget.

### Example session

Once the snippet is in place, you can just *ask*:

> **You:** "I'm starting a 3-tier web app — show me an architecture sketch
> before you write any code."
>
> **Agent:** *runs `drawpad --prompt "Does this match what you had in mind?"
> --initial proposed.json --output result.json`*
>
> *Chrome window opens with the agent's proposed boxes-and-arrows. You drag
> the database to a different layer, add a "Redis cache" rectangle, type
> "use Postgres not MySQL" in the comment, hit Send.*
>
> **Agent:** "Got it — switching to Postgres, adding the Redis cache, and
> moving the DB to the data tier. Generating the project structure now…"

That's the whole loop. One CLI call, one back-and-forth, no more
text-only guessing.

## How it works under the hood

```json
{
  "scene":          { "type": "excalidraw", "elements": [ ... ] },
  "comment":        "free-text the user typed in the comment box",
  "screenshotPath": "/abs/path/to/draw-YYYYMMDD-HHMMSS.mmm.png",
  "cancelled":      false
}
```

Exit codes:

| Code | Meaning                                                  |
| ---- | -------------------------------------------------------- |
| 0    | User clicked **Send**. Result has `scene` + screenshot.  |
| 2    | User cancelled / closed the window. `cancelled: true`.   |
| 1    | Configuration or I/O error. `stderr` explains.           |

The full flag list — written for both humans and AI agents — is in
`drawpad --help`.

## Contributing / hacking

The repo layout is bottom-up (`diagram → session → server → web → browser
→ app → cmd/drawpad`). Tests favour real integration over mocks. See
[AGENTS.md](./AGENTS.md) for the contract followed by agents working on
drawpad itself.

```bash
git clone https://github.com/merijjeyn/drawpad
cd drawpad
go test -race ./...
go run ./cmd/drawpad --prompt "hello"
```

## License

[MIT](./LICENSE)
