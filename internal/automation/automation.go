// Package automation provides interchangeable browser and desktop adapters.
package automation

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Request describes one bounded device action. Session comes from the agent, never model input.
type Request struct {
	Action         string   `json:"action"`
	URL            string   `json:"url,omitempty"`
	Selector       string   `json:"selector,omitempty"`
	Text           string   `json:"text,omitempty"`
	Key            string   `json:"key,omitempty"`
	App            string   `json:"app,omitempty"`
	Tab            string   `json:"tab,omitempty"`
	Element        int      `json:"element,omitempty"`
	X              int      `json:"x,omitempty"`
	Y              int      `json:"y,omitempty"`
	EndX           int      `json:"endX,omitempty"`
	EndY           int      `json:"endY,omitempty"`
	TargetSelector string   `json:"targetSelector,omitempty"`
	Modifiers      []string `json:"modifiers,omitempty"`
	Files          []string `json:"files,omitempty"`
	Accept         bool     `json:"accept,omitempty"`
	Session        string   `json:"-"`
}

// Result contains a bounded observation and an optional screenshot.
type Result struct {
	Text  string `json:"text"`
	Image []byte `json:"-"`
}

// Backend is the adapter seam for browsers, desktops, and future crawlers.
type Backend interface {
	Run(context.Context, Request) (Result, error)
	Close() error
}

// Registry owns adapters and closes their resources at application shutdown.
type Registry struct {
	mu       sync.RWMutex
	backends map[string]Backend
}

// NewRegistry creates an empty adapter registry.
func NewRegistry() *Registry { return &Registry{backends: make(map[string]Backend)} }

// Register installs an adapter during startup. Names must be unique.
func (r *Registry) Register(name string, backend Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.backends[name] = backend
}

// Run executes one action with a bounded timeout.
func (r *Registry) Run(ctx context.Context, name string, request Request) (Result, error) {
	r.mu.RLock()
	backend := r.backends[name]
	r.mu.RUnlock()
	if backend == nil {
		return Result{}, fmt.Errorf("automation backend %q is unavailable", name)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return backend.Run(ctx, request)
}

// Close releases every registered adapter, including after an individual failure.
func (r *Registry) Close() error {
	r.mu.RLock()
	backends := make([]Backend, 0, len(r.backends))
	for _, b := range r.backends {
		backends = append(backends, b)
	}
	r.mu.RUnlock()
	var errs []error
	for _, b := range backends {
		if err := b.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ValidateURL rejects non-web schemes and embedded credentials.
func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return fmt.Errorf("use an HTTP or HTTPS URL without embedded credentials")
	}
	return nil
}
func bounded(s string) string {
	r := []rune(s)
	if len(r) > 16000 {
		return string(r[:16000]) + "\n[observation truncated]"
	}
	return s
}
func artifact(data []byte) (string, error) {
	if len(data) > 12<<20 {
		return "", fmt.Errorf("screenshot exceeds 12 MiB")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".owncode", "automation", "screenshots")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "capture-*.png")
	if err != nil {
		return "", err
	}
	name := f.Name()
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, closeErr
}
