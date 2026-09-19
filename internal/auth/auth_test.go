package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func response(body string, status int) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func privateHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
}

func TestLoginValidatesStateBeforeExchange(t *testing.T) {
	l := newLogin("http://localhost:1455/auth/callback")
	u, err := url.Parse(l.URL)
	require.NoError(t, err)
	hash := sha256.Sum256([]byte(l.verifier))
	require.Equal(t, base64.RawURLEncoding.EncodeToString(hash[:]), u.Query().Get("code_challenge"))
	bad := httptest.NewRecorder()
	l.callback(bad, httptest.NewRequest("GET", "/auth/callback?state=wrong&code=secret-code", nil))
	require.Equal(t, http.StatusBadRequest, bad.Code)
	require.Empty(t, l.result)
	l.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, issuer+"/oauth/token", r.URL.String())
		require.NoError(t, r.ParseForm())
		require.Equal(t, l.verifier, r.Form.Get("code_verifier"))
		require.Equal(t, "valid-code", r.Form.Get("code"))
		claims := base64.RawURLEncoding.EncodeToString([]byte(`{"https://api.openai.com/auth":{"chatgpt_account_id":"account"}}`))
		return response(fmt.Sprintf(`{"access_token":"access","refresh_token":"refresh","id_token":"a.%s.c","expires_in":3600}`, claims), 200), nil
	})}
	good := httptest.NewRecorder()
	l.callback(good, httptest.NewRequest("GET", "/auth/callback?state="+l.state+"&code=valid-code", nil))
	token, err := l.Wait(context.Background())
	require.NoError(t, err)
	require.Equal(t, "account", token.Account)
	require.Equal(t, "access", token.Access)
	require.Greater(t, time.Until(token.Expires), 59*time.Minute)
	require.NotContains(t, good.Body.String(), "valid-code")
}

func TestLoginCancellationAndRedaction(t *testing.T) {
	l := newLogin("http://localhost:1455/auth/callback")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := l.Wait(ctx)
	require.ErrorIs(t, err, context.Canceled)
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return response(`{"secret":"do-not-display"}`, 401), nil })}
	_, err = exchange(context.Background(), client, url.Values{})
	require.ErrorContains(t, err, "HTTP 401")
	require.NotContains(t, err.Error(), "do-not-display")
}

func TestPrivateStoreAndHostRestriction(t *testing.T) {
	privateHome(t)
	require.NoError(t, Save(Claude, Connection{Key: "private-key"}))
	require.NoError(t, Save(ChatGPT, Connection{Token: &Token{Access: "access", Account: "account", Expires: time.Now().Add(time.Hour)}}))
	connections, err := Connections()
	require.NoError(t, err)
	require.Len(t, connections, 2)
	path, err := storePath()
	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	called := 0
	transport := &Transport{Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called++
		require.Equal(t, "Bearer access", r.Header.Get("Authorization"))
		require.Equal(t, "account", r.Header.Get("chatgpt-account-id"))
		return response(`{}`, 200), nil
	})}
	for _, endpoint := range []string{"https://third-party.test/backend-api/codex/responses", "http://chatgpt.com/backend-api/codex/responses", "https://chatgpt.com:8443/backend-api/codex/responses", "https://chatgpt.com/other"} {
		req, err := http.NewRequest("POST", endpoint, nil)
		require.NoError(t, err)
		_, err = transport.RoundTrip(req)
		require.Error(t, err)
	}
	require.Zero(t, called)
	req, err := http.NewRequest("POST", CodexURL+"responses", nil)
	require.NoError(t, err)
	res, err := transport.RoundTrip(req)
	require.NoError(t, err)
	res.Body.Close()
	require.Equal(t, 1, called)
	require.Empty(t, req.Header.Get("Authorization"))
}

func TestDiscoverFiltersChatGPTAndPaginatesClaude(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "chatgpt.com" {
			require.Equal(t, "999.0.0", r.URL.Query().Get("client_version"))
			require.Equal(t, "Bearer access", r.Header.Get("Authorization"))
			return response(`{"models":[{"slug":"hidden","visibility":"hide"},{"slug":"test-codex","display_name":"Test Codex","visibility":"list","context_window":262144,"default_reasoning_level":"high","supported_reasoning_levels":[{"effort":"low"},{"effort":"high"},{"effort":"xhigh"}]}]}`, 200), nil
		}
		require.Equal(t, "key", r.Header.Get("x-api-key"))
		if r.URL.Query().Get("after_id") == "first" {
			return response(`{"data":[{"id":"second","display_name":"Second"}]}`, 200), nil
		}
		return response(`{"data":[{"id":"first","display_name":"First"}],"has_more":true,"last_id":"first"}`, 200), nil
	})}
	models, err := discover(context.Background(), client, ChatGPT, Connection{Token: &Token{Access: "access"}})
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, int64(262144), models[0].Context)
	require.Equal(t, []string{"low", "high", "xhigh"}, models[0].ReasoningLevels)
	require.Equal(t, "high", models[0].DefaultReasoning)
	models, err = discover(context.Background(), client, Claude, Connection{Key: "key"})
	require.NoError(t, err)
	require.Len(t, models, 2)
	require.Equal(t, "second", models[1].ID)
}

func TestDiscoverDoesNotInventModelsForEmptyOrHiddenCatalog(t *testing.T) {
	for _, body := range []string{`{"models":[]}`, `{"models":[{"slug":"hidden","visibility":"hide"}]}`} {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return response(body, 200), nil })}
		catalog, err := discover(context.Background(), client, ChatGPT, Connection{Token: &Token{Access: "fake"}})
		require.ErrorContains(t, err, "none were selectable")
		require.Empty(t, catalog)
	}
}

func TestConcurrentRefreshSavesOnceAndKeepsAccount(t *testing.T) {
	privateHome(t)
	require.NoError(t, Save(ChatGPT, Connection{Token: &Token{Access: "old", Refresh: "refresh", Account: "account", Expires: time.Now().Add(-time.Hour)}}))
	var requests atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests.Add(1)
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
		if r.Form.Get("refresh_token") != "refresh" {
			return response(`{}`, 400), nil
		}
		return response(`{"access_token":"new","expires_in":3600}`, 200), nil
	})}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			token, err := currentTokenWithClient(context.Background(), client)
			if err != nil || token.Access != "new" {
				t.Errorf("refresh did not return the new token")
			}
		})
	}
	wg.Wait()
	require.Equal(t, int32(1), requests.Load())
	saved, err := Connections()
	require.NoError(t, err)
	require.Equal(t, "refresh", saved[ChatGPT].Token.Refresh)
	require.Equal(t, "account", saved[ChatGPT].Token.Account)
}
