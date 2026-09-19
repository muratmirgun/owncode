// Package auth manages provider connections independently of project configuration.
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const ChatGPT = "chatgpt"
const Claude = "anthropic"

// Token contains credentials issued by the OpenAI authorization server.
type Token struct {
	Access  string    `json:"access"`
	Refresh string    `json:"refresh"`
	Account string    `json:"account"`
	Expires time.Time `json:"expires"`
}

// Model describes an available model in a provider connection.
type Model struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Context int64  `json:"context"`
	Output  int64  `json:"output"`
}

// Connection stores credentials and the model catalog from a successful connection.
type Connection struct {
	Key    string  `json:"key,omitempty"`
	Token  *Token  `json:"token,omitempty"`
	Models []Model `json:"models"`
}

var storeMu sync.Mutex

func storePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "owncode", "connections.json"), nil
}

// Connections reads saved connections without exposing credentials in UI text.
func Connections() (map[string]Connection, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	return readStore()
}

func readStore() (map[string]Connection, error) {
	path, err := storePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]Connection{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read provider connections: %w", err)
	}
	connections := map[string]Connection{}
	if json.Unmarshal(data, &connections) != nil || connections == nil {
		return nil, fmt.Errorf("invalid provider connection file")
	}
	return connections, nil
}

// Save preserves other connections and atomically replaces the private file.
func Save(id string, connection Connection) error {
	storeMu.Lock()
	defer storeMu.Unlock()
	connections, err := readStore()
	if err != nil {
		return err
	}
	connections[id] = connection
	return writeStore(connections)
}

func writeStore(connections map[string]Connection) error {
	path, err := storePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(connections, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".connections-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
