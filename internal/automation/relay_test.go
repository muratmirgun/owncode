package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestRelayPairingAndRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := NewRelay()
	if err := r.start(); err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	endpoint := "http://" + r.address
	for _, origin := range []string{"", "https://evil.example"} {
		req, _ := http.NewRequest("GET", endpoint+"/next", nil)
		req.Header.Set("Origin", origin)
		if origin != "" {
			req.Header.Set("Authorization", "Bearer "+r.token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 401 && resp.StatusCode != 403 {
			t.Fatalf("unauthorized request returned %d", resp.StatusCode)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		result, err := r.Run(ctx, Request{Session: "worker-1", Action: "observe"})
		if err == nil && result.Text != "observed" {
			err = io.ErrUnexpectedEOF
		}
		done <- err
	}()
	req, _ := http.NewRequestWithContext(ctx, "GET", endpoint+"/next", nil)
	req.Header.Set("Authorization", "Bearer "+r.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var job relayJob
	if err = json.NewDecoder(resp.Body).Decode(&job); err != nil {
		resp.Body.Close()
		t.Fatal(err)
	}
	resp.Body.Close()
	if job.Session != "worker-1" {
		t.Fatalf("session = %s", job.Session)
	}
	data, _ := json.Marshal(relayReply{ID: job.ID, Text: "observed"})
	req, _ = http.NewRequestWithContext(ctx, "POST", endpoint+"/result", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+r.token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}
func TestRelayCanceledWithoutExtension(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := NewRelay()
	defer r.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Run(ctx, Request{Action: "observe"}); err == nil {
		t.Fatal("expected cancellation")
	}
	if len(r.pending) != 0 {
		t.Fatal("canceled request retained")
	}
}

func TestRelayCloseUnblocksQueuedWork(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := NewRelay()
	if err := r.start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := r.Run(context.Background(), Request{Action: "observe"}); done <- err }()
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("closed relay returned success")
		}
	case <-time.After(time.Second):
		t.Fatal("relay close did not release work")
	}
}
