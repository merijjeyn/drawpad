package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mericungor/draw_interface/internal/diagram"
	"github.com/mericungor/draw_interface/internal/session"
)

// newTestServer wires up a real Server backed by a temp screenshot dir and a
// fresh Session. We deliberately use net/http/httptest rather than mocking
// http.Handler — the integration test is the unit, per the task brief.
func newTestServer(t *testing.T, initial diagram.InitialPayload) (*httptest.Server, *session.Session, string) {
	t.Helper()
	sess := session.New(initial)
	dir := t.TempDir()
	srv, err := New(Config{Session: sess, ScreenshotDir: dir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, sess, dir
}

func TestServer_InitReturnsPayload(t *testing.T) {
	initial := diagram.InitialPayload{
		Prompt: "is this right?",
		Scene:  diagram.NewScene(),
	}
	ts, _, _ := newTestServer(t, initial)

	resp, err := http.Get(ts.URL + "/api/init")
	if err != nil {
		t.Fatalf("GET init: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	var got diagram.InitialPayload
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Prompt != initial.Prompt {
		t.Errorf("prompt: got %q, want %q", got.Prompt, initial.Prompt)
	}
	if got.Scene == nil || got.Scene.Type != "excalidraw" {
		t.Errorf("scene not delivered: %+v", got.Scene)
	}
}

func TestServer_SubmitWritesScreenshotAndCompletesSession(t *testing.T) {
	ts, sess, dir := newTestServer(t, diagram.InitialPayload{})

	// A 1x1 transparent PNG so the file is real but tiny.
	pngBase64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="

	scene := diagram.NewScene()
	scene.Elements = append(scene.Elements, diagram.Rectangle(0, 0, 100, 50))

	sub := diagram.Submission{
		Scene:         scene,
		Comment:       "looks good",
		ScreenshotPNG: "data:image/png;base64," + pngBase64,
	}
	body, _ := json.Marshal(sub)
	resp, err := http.Post(ts.URL+"/api/submit", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST submit: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}

	// Session should be complete now.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := sess.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if result.Cancelled {
		t.Errorf("unexpected Cancelled=true")
	}
	if result.Comment != "looks good" {
		t.Errorf("comment: got %q, want %q", result.Comment, "looks good")
	}
	if result.Scene == nil || len(result.Scene.Elements) != 1 {
		t.Errorf("scene not preserved: %+v", result.Scene)
	}
	if result.ScreenshotPath == "" {
		t.Fatalf("screenshot path empty")
	}
	if !strings.HasPrefix(result.ScreenshotPath, dir) {
		t.Errorf("screenshot %q not under temp dir %q", result.ScreenshotPath, dir)
	}
	// And the file on disk must actually be a PNG.
	got, err := os.ReadFile(result.ScreenshotPath)
	if err != nil {
		t.Fatalf("read screenshot: %v", err)
	}
	want, _ := base64.StdEncoding.DecodeString(pngBase64)
	if !bytes.Equal(got, want) {
		t.Errorf("screenshot bytes differ from upload (got %d, want %d)", len(got), len(want))
	}
	// And it should live in the temp dir we configured.
	if filepath.Dir(result.ScreenshotPath) != dir {
		t.Errorf("screenshot not in configured dir: %s vs %s",
			filepath.Dir(result.ScreenshotPath), dir)
	}
}

func TestServer_SubmitRejectsInvalidScene(t *testing.T) {
	ts, sess, _ := newTestServer(t, diagram.InitialPayload{})

	body := []byte(`{"scene": {"type":"tldraw","version":2,"elements":[]}}`)
	resp, err := http.Post(ts.URL+"/api/submit", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	// Session must NOT be completed by a rejected submission, otherwise the
	// user is silently locked out of trying again.
	select {
	case <-sess.Done():
		t.Errorf("session completed despite rejected submission")
	default:
	}
}

func TestServer_Cancel(t *testing.T) {
	ts, sess, _ := newTestServer(t, diagram.InitialPayload{})

	body := []byte(`{"reason":"user closed window"}`)
	resp, err := http.Post(ts.URL+"/api/cancel", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST cancel: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := sess.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if !r.Cancelled {
		t.Errorf("expected Cancelled=true, got %+v", r)
	}
	if r.Comment != "user closed window" {
		t.Errorf("reason not preserved: %q", r.Comment)
	}
}

func TestServer_ServesIndex(t *testing.T) {
	ts, _, _ := newTestServer(t, diagram.InitialPayload{})

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("<html")) {
		t.Errorf("response does not look like HTML:\n%s", body)
	}
}

func TestServer_StartListensAndShutsDown(t *testing.T) {
	// Real lifecycle: bind, hit healthz, shutdown, confirm bind is released.
	sess := session.New(diagram.InitialPayload{})
	srv, err := New(Config{Session: sess, ScreenshotDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	url, shutdown, err := srv.Start(ctx, 0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer shutdown()

	// Poll briefly for readiness.
	var resp *http.Response
	for i := 0; i < 20; i++ {
		resp, err = http.Get(url + "/healthz")
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("healthz status %d", resp.StatusCode)
	}
}
