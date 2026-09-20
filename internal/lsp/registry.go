package lsp

import (
	"maps"
	"sync"
)

// Registry publishes ready clients without sharing a mutable map with readers.
type Registry struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

// Snapshot returns a private copy. A nil registry contains no clients.
func (r *Registry) Snapshot() map[string]*Client {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return maps.Clone(r.clients)
}

// Store registers a client after initialization.
func (r *Registry) Store(name string, client *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.clients == nil {
		r.clients = make(map[string]*Client)
	}
	r.clients[name] = client
}

// Remove detaches a client before shutdown.
func (r *Registry) Remove(name string) *Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	client := r.clients[name]
	delete(r.clients, name)
	return client
}
