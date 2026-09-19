package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var refreshMu sync.Mutex

// Transport adds current ChatGPT credentials only to the fixed Codex endpoint.
type Transport struct{ Base http.RoundTripper }

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" || req.URL.Host != "chatgpt.com" || !strings.HasPrefix(req.URL.Path, "/backend-api/codex/") {
		return nil, fmt.Errorf("ChatGPT credentials require the official Codex endpoint")
	}
	token, err := currentToken(req.Context())
	if err != nil {
		return nil, err
	}
	clone := req.Clone(req.Context())
	setHeaders(clone, token)
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(clone)
}

func setHeaders(req *http.Request, token Token) {
	req.Header.Set("Authorization", "Bearer "+token.Access)
	req.Header.Set("originator", "owncode")
	if token.Account != "" {
		req.Header.Set("chatgpt-account-id", token.Account)
	}
}

func currentToken(ctx context.Context) (Token, error) {
	return currentTokenWithClient(ctx, authHTTPClient())
}

func currentTokenWithClient(ctx context.Context, client *http.Client) (Token, error) {
	// Serialize refreshes across the coding, title, and summary clients.
	refreshMu.Lock()
	defer refreshMu.Unlock()
	connections, err := Connections()
	if err != nil {
		return Token{}, err
	}
	c := connections[ChatGPT]
	if c.Token == nil {
		return Token{}, fmt.Errorf("ChatGPT is disconnected; open /connect")
	}
	if time.Until(c.Token.Expires) > time.Minute {
		return *c.Token, nil
	}
	if c.Token.Refresh == "" {
		return Token{}, fmt.Errorf("ChatGPT login expired; open /connect")
	}
	next, err := exchange(ctx, client, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {c.Token.Refresh}, "client_id": {clientID}})
	if err != nil {
		return Token{}, err
	}
	if next.Refresh == "" {
		next.Refresh = c.Token.Refresh
	}
	if next.Account == "" {
		next.Account = c.Token.Account
	}
	c.Token = &next
	if err := Save(ChatGPT, c); err != nil {
		return Token{}, fmt.Errorf("save refreshed login: %w", err)
	}
	return next, nil
}

// Discover validates credentials and retrieves models without generating a response.
func Discover(ctx context.Context, id string, connection Connection) ([]Model, error) {
	return discover(ctx, authHTTPClient(), id, connection)
}

func discover(ctx context.Context, client *http.Client, id string, c Connection) ([]Model, error) {
	endpoint := "https://api.anthropic.com/v1/models?limit=100"
	if id == ChatGPT {
		endpoint = CodexURL + "models?client_version=0.114.0"
	}
	if id != ChatGPT && id != Claude {
		return nil, fmt.Errorf("unsupported provider")
	}
	var result []Model
	for page := 0; page < 20; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		if id == ChatGPT {
			if c.Token == nil {
				return nil, fmt.Errorf("ChatGPT login is required")
			}
			setHeaders(req, *c.Token)
		} else {
			req.Header.Set("x-api-key", c.Key)
			req.Header.Set("anthropic-version", "2023-06-01")
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("cannot reach provider; check your connection and retry")
		}
		var catalog struct {
			Models []struct {
				ID         string `json:"slug"`
				Name       string `json:"display_name"`
				Visibility string `json:"visibility"`
				Context    int64  `json:"context_window"`
			} `json:"models"`
			Data []struct {
				ID   string `json:"id"`
				Name string `json:"display_name"`
			} `json:"data"`
			More bool   `json:"has_more"`
			Last string `json:"last_id"`
		}
		decodeErr := json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&catalog)
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("provider connection failed (HTTP %d); check your credentials and access", res.StatusCode)
		}
		if decodeErr != nil {
			return nil, fmt.Errorf("provider returned an invalid model list")
		}
		if id == ChatGPT {
			for _, m := range catalog.Models {
				if m.ID == "" || m.Visibility != "list" {
					continue
				}
				window := m.Context
				if window <= 0 {
					window = 200000
				}
				result = append(result, Model{ID: m.ID, Name: m.Name, Context: window, Output: min(16384, window)})
			}
			break
		}
		for _, m := range catalog.Data {
			if m.ID != "" {
				result = append(result, Model{ID: m.ID, Name: m.Name, Context: 200000, Output: 8192})
			}
		}
		if !catalog.More {
			break
		}
		if catalog.Last == "" || page == 19 {
			return nil, fmt.Errorf("provider model pagination is incomplete")
		}
		endpoint = "https://api.anthropic.com/v1/models?limit=100&after_id=" + url.QueryEscape(catalog.Last)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("provider returned no available models for this account")
	}
	for i := range result {
		if result[i].Name == "" {
			result[i].Name = result[i].ID
		}
	}
	return result, nil
}
