// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// SIGTERM/SIGINT cancel the context runServer watches: a request already in
// progress finishes, the cleanup is stopped, and the result is nil (exit 0).
func TestRunServerFinishesInFlightRequestsThenReturnsNil(t *testing.T) {
	started := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(300 * time.Millisecond)
		io.WriteString(w, "finished")
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cleaned := false
	result := make(chan error, 1)
	go func() {
		result <- runServer(ctx, &http.Server{Handler: mux}, ln, func() { cleaned = true }, 5*time.Second)
	}()

	body := make(chan string, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String() + "/slow")
		if err != nil {
			body <- "error: " + err.Error()
			return
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		body <- string(b)
	}()
	<-started
	cancel() // the signal

	if got := <-body; got != "finished" {
		t.Errorf("in-flight request got %q, want it to finish", got)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Errorf("runServer = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runServer did not return")
	}
	if !cleaned {
		t.Error("the scheduled cleanup was not stopped")
	}
}

func TestRunServerGivesUpAfterTimeout(t *testing.T) {
	started := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/stuck", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(2 * time.Second)
	})
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- runServer(ctx, &http.Server{Handler: mux}, ln, func() {}, 100*time.Millisecond) }()
	go http.Get("http://" + ln.Addr().String() + "/stuck")
	<-started
	cancel()
	if err := <-result; err == nil {
		t.Error("a request outliving the timeout should be reported")
	}
}

func TestCheckEnvironmentMessages(t *testing.T) {
	t.Setenv("PORT", "eighty")
	if err := checkEnvironment(); err == nil || !strings.Contains(err.Error(), "PORT") {
		t.Errorf("bad PORT: %v", err)
	}
	t.Setenv("PORT", "8080")
	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "missing", "t.db"))
	if err := checkEnvironment(); err == nil || !strings.Contains(err.Error(), "DB_PATH") {
		t.Errorf("missing folder: %v", err)
	}
	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "t.db"))
	if err := checkEnvironment(); err != nil {
		t.Errorf("good settings refused: %v", err)
	}
}
