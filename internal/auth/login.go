package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const clientID = "app_EMoamEEZ73f0CkXaXp7hrann"
const issuer = "https://auth.openai.com"
const CodexURL = "https://chatgpt.com/backend-api/codex/"

// Login owns a loopback listener for one browser authorization attempt.
type Login struct {
	URL                       string
	state, verifier, redirect string
	server                    *http.Server
	result                    chan url.Values
	client                    *http.Client
}

// StartLogin prepares a PKCE flow. The caller must close it after completion or cancellation.
func StartLogin() (*Login, error) {
	var listener net.Listener
	var err error
	for _, port := range []string{"1455", "1457"} {
		listener, err = net.Listen("tcp", "127.0.0.1:"+port)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("cannot open login callback ports 1455 or 1457")
	}
	login := newLogin("http://localhost:" + fmt.Sprint(listener.Addr().(*net.TCPAddr).Port) + "/auth/callback")
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", login.callback)
	login.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = login.server.Serve(listener) }()
	return login, nil
}

func newLogin(redirect string) *Login {
	verifier := rand.Text() + rand.Text()
	state := rand.Text()
	hash := sha256.Sum256([]byte(verifier))
	query := url.Values{
		"client_id": {clientID}, "response_type": {"code"}, "redirect_uri": {redirect},
		"scope": {"openid profile email offline_access"}, "state": {state},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(hash[:])}, "code_challenge_method": {"S256"},
		"id_token_add_organizations": {"true"}, "codex_cli_simplified_flow": {"true"}, "originator": {"owncode"},
	}
	return &Login{URL: issuer + "/oauth/authorize?" + query.Encode(), state: state, verifier: verifier, redirect: redirect,
		result: make(chan url.Values, 1), client: authHTTPClient()}
}

func authHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func (l *Login) callback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	q := r.URL.Query()
	if r.Method != http.MethodGet || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(l.state)) != 1 {
		http.Error(w, "Invalid login callback. Return to OwnCode and retry.", http.StatusBadRequest)
		return
	}
	if q.Get("code") == "" && q.Get("error") == "" {
		http.Error(w, "Missing authorization code.", http.StatusBadRequest)
		return
	}
	select {
	case l.result <- q:
		fmt.Fprint(w, "Return to OwnCode to finish connecting. You can close this tab.")
	default:
		http.Error(w, "This login callback was already received.", http.StatusConflict)
	}
}

// Wait exchanges the callback code. It does not save credentials.
func (l *Login) Wait(ctx context.Context) (Token, error) {
	select {
	case q := <-l.result:
		if q.Get("error") != "" {
			return Token{}, fmt.Errorf("login was declined or failed; retry in the browser")
		}
		return exchange(ctx, l.client, url.Values{"grant_type": {"authorization_code"}, "client_id": {clientID},
			"code": {q.Get("code")}, "code_verifier": {l.verifier}, "redirect_uri": {l.redirect}})
	case <-ctx.Done():
		return Token{}, ctx.Err()
	}
}

// Close releases the listener, including after a cancelled login.
func (l *Login) Close() {
	if l.server != nil {
		_ = l.server.Close()
	}
}

func exchange(ctx context.Context, client *http.Client, values url.Values) (Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, issuer+"/oauth/token", strings.NewReader(values.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Token{}, ctx.Err()
		}
		return Token{}, fmt.Errorf("login server unavailable; retry")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Token{}, fmt.Errorf("login token exchange failed (HTTP %d); reconnect the provider", res.StatusCode)
	}
	var data struct {
		Access  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
		ID      string `json:"id_token"`
		Expires int64  `json:"expires_in"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&data) != nil || data.Access == "" || data.Expires <= 0 {
		return Token{}, fmt.Errorf("login server returned an invalid token")
	}
	account := tokenAccount(data.ID)
	if account == "" {
		account = tokenAccount(data.Access)
	}
	return Token{Access: data.Access, Refresh: data.Refresh, Account: account, Expires: time.Now().Add(time.Duration(data.Expires) * time.Second)}, nil
}

// JWT claims supply routing metadata only. The provider verifies authorization.
func tokenAccount(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Account string `json:"chatgpt_account_id"`
		Auth    struct {
			Account string `json:"chatgpt_account_id"`
		} `json:"https://api.openai.com/auth"`
	}
	if json.Unmarshal(data, &claims) != nil {
		return ""
	}
	if claims.Auth.Account != "" {
		return claims.Auth.Account
	}
	return claims.Account
}
