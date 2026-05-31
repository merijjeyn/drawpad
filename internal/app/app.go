// Package app is the orchestrator. It wires session + server + browser into
// a single Run() call that the CLI invokes:
//
//	1. Build a Session seeded with the agent's InitialPayload.
//	2. Start the HTTP server on a random localhost port.
//	3. Launch a Chrome --app window pointed at that URL.
//	4. Wait for whichever happens first:
//	     a) the user clicks Send / Cancel  → session.Done() fires
//	     b) the user closes the window     → browser.Run() returns
//	5. Tear everything down and return the Result.
package app

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/merijjeyn/drawpad/internal/browser"
	"github.com/merijjeyn/drawpad/internal/diagram"
	"github.com/merijjeyn/drawpad/internal/server"
	"github.com/merijjeyn/drawpad/internal/session"
)

// Config bundles the inputs to one drawing interaction.
type Config struct {
	Initial       diagram.InitialPayload
	ScreenshotDir string
	BrowserOpts   browser.Options
	Logger        *log.Logger

	// Launch overrides the browser launcher. Tests use this to bypass
	// Chrome entirely. nil → browser.Open.
	Launch browser.LaunchFunc

	// Port is the localhost port to listen on. 0 → random free port.
	Port int
}

// App is a configured orchestrator. Create with New, run with Run.
type App struct {
	cfg    Config
	logger *log.Logger
}

// New validates the config and returns a ready-to-run App.
func New(cfg Config) (*App, error) {
	if cfg.ScreenshotDir == "" {
		return nil, errors.New("app: ScreenshotDir is required")
	}
	if cfg.BrowserOpts.Width == 0 && cfg.BrowserOpts.Height == 0 {
		cfg.BrowserOpts = browser.Defaults()
	}
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}
	return &App{cfg: cfg, logger: logger}, nil
}

// Run executes one drawing interaction end-to-end. It returns the user's
// result (or a cancellation marker) and nil error on success. ctx cancellation
// propagates as a cancelled result with ctx.Err() returned.
func (a *App) Run(ctx context.Context) (diagram.Result, error) {
	sess := session.New(a.cfg.Initial)

	// Internal lifetime: cancelled when we're ready to shut down, so the
	// server's background goroutine winds itself up too.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	srv, err := server.New(server.Config{
		Session:       sess,
		ScreenshotDir: a.cfg.ScreenshotDir,
		Logger:        a.logger,
	})
	if err != nil {
		return diagram.Result{}, fmt.Errorf("app: server: %w", err)
	}
	url, shutdown, err := srv.Start(runCtx, a.cfg.Port)
	if err != nil {
		return diagram.Result{}, fmt.Errorf("app: start server: %w", err)
	}
	defer shutdown()
	a.logger.Printf("draw: serving on %s", url)

	bc := browser.New(url)
	bc.Opts = a.cfg.BrowserOpts
	bc.Logger = a.logger
	if a.cfg.Launch != nil {
		bc.Launch = a.cfg.Launch
	}

	// Browser runs in its own goroutine so we can race its completion
	// against the session completion.
	browserErr := make(chan error, 1)
	go func() {
		// Use runCtx so cancelling runCtx kills the browser process.
		browserErr <- bc.Run(runCtx)
	}()

	select {
	case <-sess.Done():
		// User clicked Send or Cancel. The window is probably still open —
		// our frontend calls window.close() after submitting, but we should
		// not depend on that. Cancel the run context to kill the browser.
		cancel()
		// Drain the browser goroutine. Ignore the error since we just killed it.
		<-browserErr
		return sess.Result(), nil

	case err := <-browserErr:
		// Window was closed before any submit/cancel reached the server.
		// Treat as a cancellation so the agent sees a consistent result.
		_ = sess.Cancel("window closed")
		if err != nil && !errors.Is(err, context.Canceled) {
			a.logger.Printf("draw: browser exited with: %v", err)
		}
		return sess.Result(), nil

	case <-ctx.Done():
		cancel()
		<-browserErr
		_ = sess.Cancel("context cancelled")
		return sess.Result(), ctx.Err()
	}
}
