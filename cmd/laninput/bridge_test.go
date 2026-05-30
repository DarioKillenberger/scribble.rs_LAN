package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPostInputUsesHTTPClientTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	previousClient := lanHTTPClient
	lanHTTPClient = &http.Client{Timeout: 10 * time.Millisecond}
	t.Cleanup(func() {
		lanHTTPClient = previousClient
	})

	start := time.Now()
	err := postInput(server.URL, "token", "", keyboardInput{KeyboardID: "kbd", Key: "a", Action: "keydown"})
	if err == nil {
		t.Fatal("postInput succeeded, want timeout")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("postInput took %s, want bounded by client timeout", elapsed)
	}
}

func TestEnqueueInputDropsInsteadOfBlockingWhenQueueFull(t *testing.T) {
	t.Parallel()

	previousQueue := inputQueue
	previousDropped := droppedInputCount.Load()
	inputQueue = make(chan queuedKeyboardInput, 1)
	droppedInputCount.Store(0)
	t.Cleanup(func() {
		inputQueue = previousQueue
		droppedInputCount.Store(previousDropped)
	})

	enqueueInput(keyboardInput{KeyboardID: "kbd", Key: "a", Action: "keydown"})

	done := make(chan struct{})
	go func() {
		enqueueInput(keyboardInput{KeyboardID: "kbd", Key: "b", Action: "keydown"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("enqueueInput blocked on a full queue")
	}
	if got := droppedInputCount.Load(); got != 1 {
		t.Fatalf("dropped input count = %d, want 1", got)
	}
}
