// Package session coordinates a single drawing interaction.
//
// One Session holds the initial payload the agent provided (what to show in
// the UI), and a one-shot channel the HTTP server writes to when the user
// submits or cancels. The CLI blocks on Wait until that happens.
//
// A Session is intentionally tiny — it owns no goroutines, no I/O, no timers.
// All disk and network concerns belong to higher layers.
package session

import (
	"context"
	"errors"
	"sync"

	"github.com/mericungor/draw_interface/internal/diagram"
)

// ErrAlreadyCompleted is reported when Complete is called more than once on
// the same session. The first call wins; subsequent calls are ignored but
// reported so the server can log unexpected duplicates.
var ErrAlreadyCompleted = errors.New("session already completed")

// Session is a one-shot rendezvous between the frontend and the CLI.
type Session struct {
	initial diagram.InitialPayload

	mu        sync.Mutex
	completed bool
	result    diagram.Result
	done      chan struct{}
}

// New returns a fresh session seeded with the given initial payload.
func New(initial diagram.InitialPayload) *Session {
	return &Session{
		initial: initial,
		done:    make(chan struct{}),
	}
}

// Initial returns the payload to deliver to the frontend on first connect.
func (s *Session) Initial() diagram.InitialPayload {
	return s.initial
}

// Complete records the final result and unblocks Wait. The first call wins —
// later calls return ErrAlreadyCompleted without overwriting the result.
func (s *Session) Complete(r diagram.Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.completed {
		return ErrAlreadyCompleted
	}
	s.completed = true
	s.result = r
	close(s.done)
	return nil
}

// Cancel is a convenience for Complete with Cancelled=true.
func (s *Session) Cancel(reason string) error {
	return s.Complete(diagram.Result{Cancelled: true, Comment: reason})
}

// Wait blocks until Complete is called or ctx is done. On ctx.Err(), the
// session is automatically completed as cancelled so future Waits and the
// server's Done channel observe a consistent terminal state.
func (s *Session) Wait(ctx context.Context) (diagram.Result, error) {
	select {
	case <-s.done:
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.result, nil
	case <-ctx.Done():
		// Best-effort cancel — ignore ErrAlreadyCompleted because a Complete
		// could have raced in just before ctx.Done() fired.
		_ = s.Complete(diagram.Result{Cancelled: true, Comment: "context cancelled"})
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.result, ctx.Err()
	}
}

// Done returns a channel closed when the session completes. Useful for the
// server to learn it can shut down without polling.
func (s *Session) Done() <-chan struct{} { return s.done }

// Result returns the completed result. If called before Done() fires, the
// returned Result is zero-valued — callers should select on Done() first.
func (s *Session) Result() diagram.Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.result
}
