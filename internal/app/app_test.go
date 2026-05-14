package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mericungor/draw_interface/internal/browser"
	"github.com/mericungor/draw_interface/internal/diagram"
)

// 1x1 transparent PNG, base64-encoded — small enough to inline in tests.
const onePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="

// TestApp_EndToEnd_SubmitFlow exercises every wire in the system except the
// real browser. Instead of opening Chrome, the stub Launch function spawns a
// trivial process (so the Controller has something to wait on) and starts a
// goroutine that plays the role of the frontend: fetch /api/init, then POST
// /api/submit. The orchestrator must produce a Result whose Scene, Comment,
// and ScreenshotPath all reflect what the "frontend" sent.
func TestApp_EndToEnd_SubmitFlow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX /bin/sh as stub browser process")
	}

	initialScene := diagram.NewScene()
	initialScene.Elements = append(initialScene.Elements,
		diagram.Rectangle(10, 10, 100, 60))

	dir := t.TempDir()

	cfg := Config{
		Initial: diagram.InitialPayload{
			Prompt: "Confirm this architecture",
			Scene:  initialScene,
		},
		ScreenshotDir: dir,
	}

	frontendDone := make(chan error, 1)

	cfg.Launch = func(ctx context.Context, url string, _ browser.Options) (*exec.Cmd, error) {
		// Start a stand-in process that lives long enough for the fake
		// frontend below to interact with the server. The orchestrator
		// kills it when the session completes.
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "sleep 30")
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		go func() {
			frontendDone <- runFakeFrontend(url, initialScene.Elements[0]["id"].(string))
		}()
		return cmd, nil
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := a.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if err := <-frontendDone; err != nil {
		t.Fatalf("fake frontend failed: %v", err)
	}

	if result.Cancelled {
		t.Errorf("expected non-cancelled result, got %+v", result)
	}
	if result.Comment != "looks good to me" {
		t.Errorf("comment: got %q, want %q", result.Comment, "looks good to me")
	}
	if result.Scene == nil {
		t.Fatalf("scene is nil")
	}
	// Fake frontend should have added one element on top of the initial one.
	if got := len(result.Scene.Elements); got != 2 {
		t.Errorf("elements: got %d, want 2", got)
	}
	if result.ScreenshotPath == "" {
		t.Fatalf("screenshot path empty")
	}
	if filepath.Dir(result.ScreenshotPath) != dir {
		t.Errorf("screenshot in wrong dir: %s vs %s",
			filepath.Dir(result.ScreenshotPath), dir)
	}
	body, err := os.ReadFile(result.ScreenshotPath)
	if err != nil {
		t.Fatalf("read screenshot: %v", err)
	}
	want, _ := base64.StdEncoding.DecodeString(onePixelPNG)
	if !bytes.Equal(body, want) {
		t.Errorf("screenshot bytes differ from upload")
	}
}

// TestApp_EndToEnd_WindowClosedWithoutSubmit verifies that closing the
// browser window (= the launch'd process exiting) is reported back to the
// agent as a clean cancellation, not as a hard error.
func TestApp_EndToEnd_WindowClosedWithoutSubmit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX /bin/sh as stub browser process")
	}
	dir := t.TempDir()
	cfg := Config{
		Initial:       diagram.InitialPayload{Prompt: "test"},
		ScreenshotDir: dir,
		Launch: func(ctx context.Context, url string, _ browser.Options) (*exec.Cmd, error) {
			// Process exits ~immediately, simulating the user closing the
			// window before doing anything.
			cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "sleep 0.1")
			if err := cmd.Start(); err != nil {
				return nil, err
			}
			return cmd, nil
		},
	}
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, err := a.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !r.Cancelled {
		t.Errorf("expected Cancelled=true after window closed, got %+v", r)
	}
}

// runFakeFrontend impersonates the JS in app.js — fetches init, then submits.
func runFakeFrontend(url, initialElementID string) error {
	// Wait briefly for the server to be ready. We poll /healthz instead of
	// sleeping a fixed duration so the test is robust against slow CI.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if r, err := http.Get(url + "/healthz"); err == nil {
			r.Body.Close()
			if r.StatusCode == 200 {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Fetch the initial payload (would normally drive the React mount).
	initResp, err := http.Get(url + "/api/init")
	if err != nil {
		return fmt.Errorf("GET init: %w", err)
	}
	defer initResp.Body.Close()
	var init diagram.InitialPayload
	if err := json.NewDecoder(initResp.Body).Decode(&init); err != nil {
		return fmt.Errorf("decode init: %w", err)
	}
	if init.Scene == nil || len(init.Scene.Elements) != 1 {
		return fmt.Errorf("init did not include initial scene: %+v", init.Scene)
	}
	if id := init.Scene.Elements[0]["id"]; id != initialElementID {
		return fmt.Errorf("element id mismatch: got %v, want %v", id, initialElementID)
	}

	// User adds a second element and types a comment, then hits Send.
	scene := init.Scene
	scene.Elements = append(scene.Elements, diagram.Ellipse(150, 10, 100, 60))

	sub := diagram.Submission{
		Scene:         scene,
		Comment:       "looks good to me",
		ScreenshotPNG: "data:image/png;base64," + onePixelPNG,
	}
	body, _ := json.Marshal(sub)
	r, err := http.Post(url+"/api/submit", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("POST submit: %w", err)
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return fmt.Errorf("submit status %d", r.StatusCode)
	}
	return nil
}

// Helps debugging: confirm we can stand the whole stack up against random ports.
func TestApp_New_ValidatesConfig(t *testing.T) {
	if _, err := New(Config{}); err == nil || !strings.Contains(err.Error(), "ScreenshotDir") {
		t.Errorf("expected ScreenshotDir error, got %v", err)
	}
}
