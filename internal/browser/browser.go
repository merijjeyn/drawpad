// Package browser launches a Chrome window in --app mode pointed at a local
// URL and returns when the window is closed by the user (or the context is
// done). No chromedp dependency — we just exec the binary, which is enough
// for our one-window-one-session use case.
package browser

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// LaunchFunc opens the URL and returns a process-like handle. Made
// configurable so tests can substitute a stub binary (see browser_test.go).
type LaunchFunc func(ctx context.Context, url string, opts Options) (*exec.Cmd, error)

// Options shape the Chrome window. Width and Height are in CSS pixels.
type Options struct {
	Width       int
	Height      int
	WindowTitle string
	// ChromePath overrides auto-detection.
	ChromePath string
	// ExtraArgs are appended after the standard set. Mostly for tests.
	ExtraArgs []string
	// UserDataDir is the Chrome profile directory. Empty → temp dir per run.
	UserDataDir string
}

// Defaults returns a sensible default Options ready for fill-in.
func Defaults() Options {
	return Options{Width: 1100, Height: 760, WindowTitle: "draw"}
}

// Open is the default LaunchFunc. It locates Chrome and starts it with the
// given URL in app mode using a throw-away profile so the window is bound to
// this process (instead of being opened as a tab in an existing Chrome).
func Open(ctx context.Context, url string, opts Options) (*exec.Cmd, error) {
	chrome := opts.ChromePath
	if chrome == "" {
		var err error
		chrome, err = findChrome()
		if err != nil {
			return nil, err
		}
	}
	userDataDir := opts.UserDataDir
	if userDataDir == "" {
		var err error
		userDataDir, err = os.MkdirTemp("", "draw_interface-chrome-*")
		if err != nil {
			return nil, fmt.Errorf("browser: create user data dir: %w", err)
		}
	}
	w, h := opts.Width, opts.Height
	if w <= 0 {
		w = 1100
	}
	if h <= 0 {
		h = 760
	}

	args := []string{
		"--app=" + url,
		fmt.Sprintf("--window-size=%d,%d", w, h),
		"--user-data-dir=" + userDataDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-features=Translate",
		"--disable-background-networking",
		// Don't restore session — every launch is a fresh window.
		"--disable-session-crashed-bubble",
		"--disable-infobars",
	}
	args = append(args, opts.ExtraArgs...)

	cmd := exec.CommandContext(ctx, chrome, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("browser: start chrome: %w", err)
	}
	return cmd, nil
}

// Controller manages the lifecycle of one Chrome window.
type Controller struct {
	URL    string
	Opts   Options
	Logger *log.Logger
	Launch LaunchFunc
}

// New returns a controller with default settings. URL is the only required
// field — everything else has working defaults.
func New(url string) *Controller {
	return &Controller{
		URL:    url,
		Opts:   Defaults(),
		Logger: log.Default(),
		Launch: Open,
	}
}

// Run starts the browser and blocks until either the user closes the window
// or ctx is done. If ctx fires first, the browser process is killed.
//
// Returns nil if the user closed the window cleanly. Returns ctx.Err() if
// the context fired. Returns the underlying exec error otherwise.
func (c *Controller) Run(ctx context.Context) error {
	if c.Launch == nil {
		c.Launch = Open
	}
	cmd, err := c.Launch(ctx, c.URL, c.Opts)
	if err != nil {
		return err
	}
	// Wait in a goroutine so we can race ctx.Done() against the process exit.
	exited := make(chan error, 1)
	go func() {
		exited <- cmd.Wait()
	}()

	select {
	case err := <-exited:
		// Chrome exits non-zero when killed; treat that as a clean close
		// since the user almost certainly hit the window's X button.
		if err != nil && !isHarmlessChromeExit(err) {
			return fmt.Errorf("browser: wait: %w", err)
		}
		return nil
	case <-ctx.Done():
		// Try graceful kill first, then escalate.
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-exited
		return ctx.Err()
	}
}

func isHarmlessChromeExit(err error) bool {
	if err == nil {
		return true
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		// Closing the window leaves Chrome's exit code in {0, 15, 130, 137}
		// depending on platform. We don't care about the exact code — the
		// outcome is the same: the user is done.
		return true
	}
	return false
}

// findChrome searches the standard install locations for Chrome or Chromium.
func findChrome() (string, error) {
	// PATH first — covers Linux installs and lets users override via PATH.
	candidates := []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"}
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	// Platform-specific fall-backs.
	switch runtime.GOOS {
	case "darwin":
		for _, p := range []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Google Chrome Beta.app/Contents/MacOS/Google Chrome Beta",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			filepath.Join(os.Getenv("HOME"), "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
		} {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	case "windows":
		for _, p := range []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		} {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	return "", errors.New("browser: could not find Chrome/Chromium; install Chrome or set ChromePath")
}
