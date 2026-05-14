// Command draw opens an Excalidraw window so a human can review or edit a
// diagram, then prints the resulting scene + screenshot path as JSON on
// stdout.
//
// Intended invocation from an AI agent:
//
//	draw \
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

	"github.com/mericungor/draw_interface/internal/app"
	"github.com/mericungor/draw_interface/internal/browser"
	"github.com/mericungor/draw_interface/internal/diagram"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		var ce *cancelledError
		if errors.As(err, &ce) {
			// User cancelled. Result JSON is still written; signal via exit code.
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "draw: %v\n", err)
		os.Exit(1)
	}
}

type cancelledError struct{}

func (*cancelledError) Error() string { return "cancelled" }

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("draw", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		promptFlag        = fs.String("prompt", "", "Question/instruction shown to the user in the window banner")
		initialFlag       = fs.String("initial", "", "Path to an initial Excalidraw scene JSON file (- for stdin)")
		outputFlag        = fs.String("output", "-", "Where to write the result JSON; - for stdout")
		screenshotDirFlag = fs.String("screenshot-dir", "", "Directory for the diagram screenshot (default: ./.draw-screenshots)")
		widthFlag         = fs.Int("width", 1100, "Window width in pixels")
		heightFlag        = fs.Int("height", 760, "Window height in pixels")
		timeoutFlag       = fs.Duration("timeout", 0, "Maximum time to wait for the user (0 = no limit)")
		quietFlag         = fs.Bool("quiet", false, "Suppress informational log lines on stderr")
	)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: draw [flags]\n\nFlags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
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

	a, err := app.New(app.Config{
		Initial:       initial,
		ScreenshotDir: screenshotDir,
		BrowserOpts: browser.Options{
			Width:       *widthFlag,
			Height:      *heightFlag,
			WindowTitle: "draw",
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
	logger.Printf("draw: interaction finished in %s (cancelled=%v)",
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
