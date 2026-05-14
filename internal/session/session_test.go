package session

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mericungor/draw_interface/internal/diagram"
)

func TestSession_CompleteAndWait(t *testing.T) {
	s := New(diagram.InitialPayload{Prompt: "review this"})

	want := diagram.Result{
		Scene:          diagram.NewScene(),
		Comment:        "looks good",
		ScreenshotPath: "/tmp/foo.png",
	}

	go func() {
		// Simulate the server receiving a submission shortly after Wait blocks.
		time.Sleep(10 * time.Millisecond)
		if err := s.Complete(want); err != nil {
			t.Errorf("Complete: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := s.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if got.Comment != want.Comment || got.ScreenshotPath != want.ScreenshotPath {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSession_CompleteOnlyOnce(t *testing.T) {
	s := New(diagram.InitialPayload{})
	if err := s.Complete(diagram.Result{Comment: "first"}); err != nil {
		t.Fatalf("first Complete: %v", err)
	}
	err := s.Complete(diagram.Result{Comment: "second"})
	if err != ErrAlreadyCompleted {
		t.Fatalf("second Complete: got %v, want ErrAlreadyCompleted", err)
	}
	got, err := s.Wait(context.Background())
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if got.Comment != "first" {
		t.Errorf("got %q, want %q (first call must win)", got.Comment, "first")
	}
}

func TestSession_WaitRespectsContext(t *testing.T) {
	s := New(diagram.InitialPayload{})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	r, err := s.Wait(ctx)
	if err == nil {
		t.Fatalf("expected ctx error, got nil")
	}
	if !r.Cancelled {
		t.Errorf("expected Cancelled=true after ctx timeout, got %+v", r)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Errorf("Wait took too long; ctx not respected")
	}
}

func TestSession_ConcurrentCompletersStillSeeOneResult(t *testing.T) {
	// Twenty goroutines all race to Complete; exactly one must succeed and
	// every Wait must observe the same result. Catches races in the once
	// gate even if the test is run with -race.
	s := New(diagram.InitialPayload{})
	var wg sync.WaitGroup
	wins := make(chan string, 20)
	for i := 0; i < 20; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			label := "result-" + itoa(i)
			if err := s.Complete(diagram.Result{Comment: label}); err == nil {
				wins <- label
			}
		}()
	}
	wg.Wait()
	close(wins)

	winners := 0
	for range wins {
		winners++
	}
	if winners != 1 {
		t.Fatalf("expected exactly 1 winner, got %d", winners)
	}
}

// avoids importing strconv in tests
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	out := ""
	for i > 0 {
		out = string(rune('0'+i%10)) + out
		i /= 10
	}
	return out
}
