package automation

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type relayJob struct {
	ID      string  `json:"id"`
	Request Request `json:"request"`
	Session string  `json:"session"`
	Expires int64   `json:"expires"`
	reply   chan relayReply
	ctx     context.Context
}
type relayReply struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Image string `json:"image,omitempty"`
	Error string `json:"error,omitempty"`
}

// Relay connects a paired browser extension through an authenticated loopback server.
type Relay struct {
	mu       sync.Mutex
	server   *http.Server
	jobs     chan *relayJob
	pending  map[string]*relayJob
	token    string
	address  string
	done     chan struct{}
	closed   bool
	shutdown chan struct{}
}

// NewRelay creates an inactive extension bridge.
func NewRelay() *Relay {
	return &Relay{jobs: make(chan *relayJob), pending: make(map[string]*relayJob), shutdown: make(chan struct{})}
}
func (r *Relay) start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return fmt.Errorf("browser relay is closed")
	}
	if r.server != nil {
		return nil
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	token := make([]byte, 32)
	if _, err = rand.Read(token); err != nil {
		_ = listener.Close()
		return err
	}
	r.token = hex.EncodeToString(token)
	r.address = listener.Addr().String()
	home, err := os.UserHomeDir()
	if err != nil {
		_ = listener.Close()
		return err
	}
	dir := filepath.Join(home, ".owncode")
	if err = os.MkdirAll(dir, 0700); err != nil {
		_ = listener.Close()
		return err
	}
	data, _ := json.Marshal(map[string]string{"url": "http://" + r.address, "token": r.token})
	file, err := os.CreateTemp(dir, ".browser-pair-*")
	if err != nil {
		_ = listener.Close()
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(file.Name())
		_ = listener.Close()
		return fmt.Errorf("write browser pairing: %v %v", writeErr, closeErr)
	}
	if err = os.Rename(file.Name(), filepath.Join(dir, "browser-pairing.json")); err != nil {
		_ = os.Remove(file.Name())
		_ = listener.Close()
		return err
	}
	r.server = &http.Server{Handler: http.HandlerFunc(r.handle), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 30 * time.Second}
	r.done = make(chan struct{})
	server := r.server
	done := r.done
	go func() { defer close(done); _ = server.Serve(listener) }()
	return nil
}
func (r *Relay) handle(w http.ResponseWriter, req *http.Request) {
	if req.Host != r.address {
		http.Error(w, "invalid host", http.StatusForbidden)
		return
	}
	origin := req.Header.Get("Origin")
	if origin != "" && !strings.HasPrefix(origin, "chrome-extension://") {
		http.Error(w, "invalid origin", http.StatusForbidden)
		return
	}
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
	if req.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.Header.Get("Authorization")), []byte("Bearer "+r.token)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch {
	case req.Method == http.MethodGet && req.URL.Path == "/health":
		_, _ = w.Write([]byte(`{"ready":true}`))
	case req.Method == http.MethodGet && req.URL.Path == "/next":
		ctx, cancel := context.WithTimeout(req.Context(), 12*time.Second)
		defer cancel()
		for {
			select {
			case job := <-r.jobs:
				if job.ctx.Err() != nil {
					continue
				}
				_ = json.NewEncoder(w).Encode(job)
				return
			case <-ctx.Done():
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
	case req.Method == http.MethodPost && req.URL.Path == "/result":
		var result relayReply
		if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 18<<20)).Decode(&result); err != nil {
			http.Error(w, "invalid result", 400)
			return
		}
		r.mu.Lock()
		job := r.pending[result.ID]
		r.mu.Unlock()
		if job == nil {
			http.Error(w, "expired action", 410)
			return
		}
		select {
		case job.reply <- result:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, req)
	}
}

// Run starts pairing on demand and waits for an authenticated extension response.
func (r *Relay) Run(ctx context.Context, request Request) (Result, error) {
	if err := r.start(); err != nil {
		return Result{}, err
	}
	if request.Action == "status" {
		return Result{Text: "Relay started. Pair the extension using ~/.owncode/browser-pairing.json. Never paste its token into chat."}, nil
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return Result{}, err
	}
	job := &relayJob{ID: hex.EncodeToString(id), Request: request, Session: request.Session, Expires: time.Now().Add(25 * time.Second).UnixMilli(), reply: make(chan relayReply, 1), ctx: ctx}
	r.mu.Lock()
	r.pending[job.ID] = job
	r.mu.Unlock()
	defer func() { r.mu.Lock(); delete(r.pending, job.ID); r.mu.Unlock() }()
	select {
	case r.jobs <- job:
	case <-r.shutdown:
		return Result{}, fmt.Errorf("browser relay closed")
	case <-ctx.Done():
		return Result{}, fmt.Errorf("browser extension unavailable or request canceled: %w", ctx.Err())
	}
	select {
	case <-r.shutdown:
		return Result{}, fmt.Errorf("browser relay closed")
	case reply := <-job.reply:
		if reply.Error != "" {
			return Result{}, fmt.Errorf("browser extension: %s", bounded(reply.Error))
		}
		result := Result{Text: bounded(reply.Text)}
		if reply.Image != "" {
			data, err := base64.StdEncoding.DecodeString(reply.Image)
			if err != nil {
				return Result{}, err
			}
			if len(data) > 12<<20 {
				return Result{}, fmt.Errorf("screenshot exceeds 12 MiB")
			}
			path, err := artifact(data)
			if err != nil {
				return Result{}, err
			}
			result.Image = data
			result.Text = "Screenshot: " + path
		}
		return result, nil
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
}

// Close stops the listener and releases pending callers.
func (r *Relay) Close() error {
	r.mu.Lock()
	if !r.closed {
		close(r.shutdown)
	}
	r.closed = true
	server, done := r.server, r.done
	r.mu.Unlock()
	if server == nil {
		return nil
	}
	err := server.Close()
	<-done
	return err
}
