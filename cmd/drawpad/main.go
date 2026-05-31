// Command drawpad opens an Excalidraw window so a human can review or edit a
// diagram, then prints the resulting scene + screenshot path as JSON on
// stdout.
//
// Intended invocation from an AI agent:
//
//	drawpad \
//	  --prompt   "Does this match what you had in mind?" \
//	  --initial  path/to/initial.json \
//	  --output   path/to/result.json \
//	  --screenshot-dir ./screenshots
//
// Anything not specified on the command line falls back to a working default
// (blank canvas, stdout for result JSON, ./.draw-screenshots dir).
//
// Exit codes:
//
//	0  user clicked Send — result.json (or stdout) holds Scene + screenshot
//	2  user cancelled or closed the window — Cancelled=true in the result
//	1  any other failure (bad initial JSON, can't launch Chrome, …)
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/merijjeyn/drawpad/internal/app"
	"github.com/merijjeyn/drawpad/internal/browser"
	"github.com/merijjeyn/drawpad/internal/diagram"
)

// Build metadata. Overridden at link time by GoReleaser's ldflags
// (-X main.version=… -X main.commit=… -X main.date=…). Defaults below are
// what you'll see on a `go install` or `go build` of a tagged source tree.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// `drawpad uninstall …` is a built-in subcommand that removes the
	// binary + reverses the install script's PATH edits. Routed here
	// before the regular flag parser so the subcommand's own flags
	// (--keep-path-edits etc.) don't collide with the canvas flags.
	if len(os.Args) >= 2 && os.Args[1] == "uninstall" {
		if err := runUninstall(os.Args[2:], os.Stdout, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "drawpad: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		// -h / --help: Go's flag pkg returns ErrHelp after printing Usage.
		// That's a successful invocation, not a failure.
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		var ce *cancelledError
		if errors.As(err, &ce) {
			// User cancelled. Result JSON is still written; signal via exit code.
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "drawpad: %v\n", err)
		os.Exit(1)
	}
}

type cancelledError struct{}

func (*cancelledError) Error() string { return "cancelled" }

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("drawpad", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		promptFlag        = fs.String("prompt", "", "Question/instruction shown to the user in the window banner")
		initialFlag       = fs.String("initial", "", "Path to an initial Excalidraw scene JSON file (- for stdin)")
		outputFlag        = fs.String("output", "-", "Where to write the result JSON; - for stdout")
		screenshotDirFlag = fs.String("screenshot-dir", "", "Directory for the diagram screenshot (default: ./.draw-screenshots)")
		widthFlag         = fs.Int("width", 1100, "Window width in pixels (capped at 1400)")
		heightFlag        = fs.Int("height", 760, "Window height in pixels (capped at 900)")
		timeoutFlag       = fs.Duration("timeout", 0, "Maximum time to wait for the user (0 = no limit)")
		quietFlag         = fs.Bool("quiet", false, "Suppress informational log lines on stderr")
		versionFlag       = fs.Bool("version", false, "Print version and exit")
	)
	fs.Usage = func() { fmt.Fprint(stderr, helpText) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *versionFlag {
		fmt.Fprintf(stdout, "drawpad %s (commit %s, built %s)\n", version, commit, date)
		return nil
	}

	logger := log.New(stderr, "", log.LstdFlags)
	if *quietFlag {
		logger = log.New(io.Discard, "", 0)
	}

	initial := diagram.InitialPayload{Prompt: *promptFlag}
	if *initialFlag != "" {
		scene, err := loadInitial(*initialFlag, stdin)
		if err != nil {
			return fmt.Errorf("load initial scene: %w", err)
		}
		initial.Scene = scene
	}

	screenshotDir := *screenshotDirFlag
	if screenshotDir == "" {
		screenshotDir = ".draw-screenshots"
	}
	abs, err := filepath.Abs(screenshotDir)
	if err == nil {
		screenshotDir = abs
	}

	// Hard-cap the window so an over-eager agent can't spawn a Chrome
	// window larger than fits on a typical laptop screen.
	const (
		maxWidth  = 1400
		maxHeight = 900
	)
	w, h := *widthFlag, *heightFlag
	if w > maxWidth {
		w = maxWidth
	}
	if h > maxHeight {
		h = maxHeight
	}

	a, err := app.New(app.Config{
		Initial:       initial,
		ScreenshotDir: screenshotDir,
		BrowserOpts: browser.Options{
			Width:       w,
			Height:      h,
			WindowTitle: "drawpad",
		},
		Logger: logger,
	})
	if err != nil {
		return err
	}

	// Ctrl-C in the agent's terminal should cleanly cancel the session.
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if *timeoutFlag > 0 {
		var cancelT context.CancelFunc
		ctx, cancelT = context.WithTimeout(ctx, *timeoutFlag)
		defer cancelT()
	}

	start := time.Now()
	result, err := a.Run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	logger.Printf("drawpad: interaction finished in %s (cancelled=%v)",
		time.Since(start).Round(time.Millisecond), result.Cancelled)

	if err := writeResult(*outputFlag, result); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	if result.Cancelled {
		return &cancelledError{}
	}
	return nil
}

func loadInitial(path string, stdin io.Reader) (*diagram.Scene, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	return diagram.ParseScene(data)
}

// helpText is shown for `drawpad -h` / `drawpad --help`. It is written to be
// useful for both a human skimming the terminal AND an AI agent piping it
// into its context window before invoking the tool, so it includes the
// flag list, the JSON shapes on both sides, exit codes, and examples.
const helpText = `drawpad — open an Excalidraw window for AI ↔ human collaboration on a diagram.

WHAT IT DOES
  Spawns a small Chrome window with an Excalidraw canvas pre-loaded with a
  scene you provide. The user edits, scribbles, types a free-text comment,
  then clicks Send. The CLI prints (or writes) the final scene as JSON and
  saves a PNG screenshot of the diagram, then exits. While the window is
  open the CLI blocks.

WHEN TO REACH FOR IT (AI AGENTS)
  Use drawpad whenever a picture moves the conversation forward faster than
  prose. Good fits:
    • Showcasing a proposed architecture before writing code
    • Asking the user to react to a UI / UX layout or wireframe
    • Representing a flow, sequence, or state machine for review
    • Letting the user red-line / annotate a design you drafted
    • Inviting the user to sketch what they want ("draw the flow")
  Don't use it for: trivial confirmations, anything that fits in one line
  of text, or when no display is available.

USAGE
  drawpad [flags]
  drawpad uninstall [--keep-path-edits]    # remove drawpad from your system

FLAGS
  -prompt STR         Question shown above the canvas (e.g. "Does this
                      match what you had in mind?"). Optional.
  -initial PATH       Excalidraw scene JSON to seed the canvas; "-" reads
                      stdin. Omit for a blank canvas.
  -output PATH        Where to write the result JSON; "-" = stdout (default).
  -screenshot-dir DIR Folder for the PNG export (default ./.draw-screenshots).
  -width N            Window width in pixels (default 1100, max 1400).
  -height N           Window height in pixels (default 760, max 900).
  -timeout DUR        Max time to wait for the user; Go duration syntax
                      (e.g. 5m, 1h). 0 = no limit. Default 0.
  -quiet              Silence informational log lines on stderr.
  -version            Print version and exit.
  -h, --help          Print this help and exit.

INPUT — initial scene JSON
  Matches Excalidraw's File > Save format. Minimal valid input:
    {"type":"excalidraw","version":2,"elements":[],"appState":{}}
  Each element needs at minimum {"id":"...","type":"rectangle|ellipse|
  diamond|text|arrow|line|...", ...}. Unknown fields round-trip untouched,
  so anything Excalidraw accepts is fine. See testdata/architecture.json
  and testdata/webapp_ui.json for worked examples.

  You do not need to hand-author this JSON. The internal/diagram package
  ships ergonomic helpers — Rectangle, Ellipse, Diamond, Text, Arrow,
  Line, BindText, BindArrow — and cmd/build-architecture is a complete
  example of generating a scene programmatically.

OUTPUT — result JSON
  {
    "scene":          <Excalidraw scene as the user finalized it>,
    "comment":        "free-text the user typed in the comment box",
    "screenshotPath": "/abs/path/to/screenshots/draw-YYYYMMDD-HHMMSS.mmm.png",
    "cancelled":      false
  }
  The screenshot is a PNG export of the diagram (NOT the surrounding UI
  chrome). Read it with your file tool to actually see what the user
  produced. When cancelled is true the user closed the window without
  sending and scene/screenshotPath may be empty.

EXIT CODES
  0  user clicked Send                   → result holds Scene + screenshot
  2  user cancelled / closed the window  → cancelled=true in the result
  1  configuration or I/O error          → stderr explains why

EXAMPLES
  # Ask the user to review a diagram you generated.
  drawpad --prompt "Does this match what you had in mind?" \
          --initial proposed.json \
          --output  result.json

  # Hand the user a blank canvas to sketch on.
  drawpad --prompt "Sketch the flow you want me to implement."

  # Pipe a generator straight in, capture result to a file.
  go run ./cmd/build-architecture \
    | drawpad --initial - --output review.json

TIPS FOR AGENTS
  • Treat the "comment" field as the user's primary natural-language reply.
    If it is empty, the diagram itself IS the reply — read the screenshot.
  • Exit code 2 with cancelled=true means the user bailed. Do not pretend
    they confirmed; fall back to asking in text.
  • The screenshotPath is absolute. You can hand it to a Read tool exactly
    as-is.
  • Loading the bundled Excalidraw assets requires internet (unpkg.com).
    If the network is restricted the canvas will fail to render — fall
    back to text.

`

func writeResult(path string, r diagram.Result) error {
	out, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	if path == "-" {
		_, err := os.Stdout.Write(out)
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
