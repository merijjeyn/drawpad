// Package server hosts the local HTTP endpoints the Excalidraw frontend
// talks to. It is intentionally bound to 127.0.0.1 only — nothing here is
// safe to expose on a public interface.
//
// Endpoints:
//
//	GET  /                — serves the Excalidraw UI (index.html)
//	GET  /static/*        — bundled JS/CSS
//	GET  /api/init        — initial scene + prompt for the agent
//	POST /api/submit      — user clicked Send; writes screenshot, completes session
//	POST /api/cancel      — user closed/cancelled; completes session as cancelled
//	GET  /healthz         — liveness probe (used by the orchestrator)
package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mericungor/draw_interface/internal/diagram"
	"github.com/mericungor/draw_interface/internal/session"
)

//go:embed web
var webFS embed.FS

// Config controls the server's behavior. ScreenshotDir must exist and be
// writable; the server writes one PNG per submission and reports its path
// back through the session result.
type Config struct {
	Session       *session.Session
	ScreenshotDir string
	// Logger receives operational messages. nil → log.Default().
	Logger *log.Logger
}

// Server is the HTTP layer of the drawing interaction.
type Server struct {
	cfg    Config
	logger *log.Logger
	mux    *http.ServeMux
}

// New constructs a server. It does not bind a port — use Start for that, or
// Handler() for tests that want to drive the mux directly.
func New(cfg Config) (*Server, error) {
	if cfg.Session == nil {
		return nil, errors.New("server: Session is required")
	}
	if cfg.ScreenshotDir == "" {
		return nil, errors.New("server: ScreenshotDir is required")
	}
	if err := os.MkdirAll(cfg.ScreenshotDir, 0o755); err != nil {
		return nil, fmt.Errorf("server: create screenshot dir: %w", err)
	}
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}
	s := &Server{cfg: cfg, logger: logger}
	s.mux = s.routes()
	return s, nil
}

// Handler returns the raw http.Handler — useful for httptest.
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()

	// Static frontend.
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		// embed always succeeds for declared directories, so this is a bug.
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/api/init", s.handleInit)
	mux.HandleFunc("/api/submit", s.handleSubmit)
	mux.HandleFunc("/api/cancel", s.handleCancel)
	return mux
}

// Start binds to 127.0.0.1:0 (or the requested port), serves until ctx is
// done, and returns the chosen URL once listening. The returned shutdown
// func performs a graceful close with a short timeout — call it from defer.
func (s *Server) Start(ctx context.Context, port int) (url string, shutdown func() error, err error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return "", nil, fmt.Errorf("server listen: %w", err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	url = fmt.Sprintf("http://127.0.0.1:%d", addr.Port)

	hs := &http.Server{
		Handler:      s.mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	go func() {
		if err := hs.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.logger.Printf("server: serve error: %v", err)
		}
	}()
	go func() {
		<-ctx.Done()
		_ = hs.Shutdown(context.Background())
	}()
	shutdown = func() error {
		sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return hs.Shutdown(sctx)
	}
	return url, shutdown, nil
}

// --- handlers ---------------------------------------------------------------

func (s *Server) handleInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(s.cfg.Session.Initial()); err != nil {
		s.logger.Printf("server: init encode: %v", err)
	}
}

func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Cap the body. A typical Excalidraw scene is <100KB even with many
	// shapes, but PNG screenshots inflate things — allow up to 32 MiB.
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	defer r.Body.Close()

	var sub diagram.Submission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, fmt.Sprintf("decode submission: %v", err), http.StatusBadRequest)
		return
	}
	if sub.Scene == nil {
		http.Error(w, "scene is required", http.StatusBadRequest)
		return
	}
	if err := sub.Scene.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("invalid scene: %v", err), http.StatusBadRequest)
		return
	}

	screenshotPath := ""
	if png, err := sub.DecodeScreenshot(); err != nil {
		http.Error(w, fmt.Sprintf("decode screenshot: %v", err), http.StatusBadRequest)
		return
	} else if len(png) > 0 {
		// Filename is the submission timestamp; this naturally collates and
		// makes it obvious which screenshot belongs to which session.
		name := fmt.Sprintf("draw-%s.png", time.Now().UTC().Format("20060102-150405.000"))
		full := filepath.Join(s.cfg.ScreenshotDir, name)
		if err := os.WriteFile(full, png, 0o644); err != nil {
			http.Error(w, fmt.Sprintf("write screenshot: %v", err),
				http.StatusInternalServerError)
			return
		}
		abs, err := filepath.Abs(full)
		if err != nil {
			abs = full
		}
		screenshotPath = abs
	}

	result := diagram.Result{
		Scene:          sub.Scene,
		Comment:        sub.Comment,
		ScreenshotPath: screenshotPath,
	}
	if err := s.cfg.Session.Complete(result); err != nil {
		// Duplicate submission — frontend probably double-clicked Send. Don't
		// 500; the original is already on its way back to the agent.
		s.logger.Printf("server: duplicate submission ignored: %v", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":             true,
		"screenshotPath": screenshotPath,
	})
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	// Best-effort body parse; cancellation works even with no body.
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.cfg.Session.Cancel(body.Reason); err != nil {
		s.logger.Printf("server: cancel after complete: %v", err)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
