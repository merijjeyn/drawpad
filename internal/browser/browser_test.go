package browser

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// We can't launch real Chrome in CI, but the Controller's contract — wait
// for the process to exit, kill it on ctx cancel — is testable with /bin/sh
// standing in for Chrome.

func TestController_ReturnsWhenProcessExits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX /bin/sh")
	}
	c := New("http://example.invalid/")
	c.Launch = func(ctx context.Context, url string, _ Options) (*exec.Cmd, error) {
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "sleep 0.05; exit 0")
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return cmd, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := time.Now()
	if err := c.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("Run took too long: %v (expected ~50ms)", elapsed)
	}
}

func TestController_KillsProcessOnContextDone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX /bin/sh")
	}
	c := New("http://example.invalid/")
	c.Launch = func(ctx context.Context, url string, _ Options) (*exec.Cmd, error) {
		// 30-second sleep — must be killed by ctx, not its own exit.
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "sleep 30")
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return cmd, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := c.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("kill took too long: %v", elapsed)
	}
}

func TestController_LaunchErrorBubblesUp(t *testing.T) {
	c := New("http://example.invalid/")
	c.Launch = func(ctx context.Context, url string, _ Options) (*exec.Cmd, error) {
		return nil, errors.New("boom")
	}
	err := c.Run(context.Background())
	if err == nil || err.Error() != "boom" {
		t.Errorf("got %v, want boom", err)
	}
}

func TestOpen_RejectsMissingChrome(t *testing.T) {
	// Force the path lookup to fail by pointing it at a non-existent binary.
	_, err := Open(context.Background(), "http://example.invalid/", Options{
		ChromePath: "/this/path/does/not/exist/chrome",
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestFindChrome_FindsOrReportsClearly(t *testing.T) {
	p, err := findChrome()
	if err != nil {
		// On a CI machine without Chrome this is expected — make sure the
		// error message tells the user what's wrong.
		if err.Error() == "" {
			t.Errorf("error has no message")
		}
		return
	}
	if _, statErr := os.Stat(p); statErr != nil {
		t.Errorf("findChrome returned %q but stat: %v", p, statErr)
	}
}
