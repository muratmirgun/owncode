package automation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBrowserIntegration(t *testing.T) {
	if os.Getenv("OWNCODE_BROWSER_TEST") != "1" {
		t.Skip("set OWNCODE_BROWSER_TEST=1 to run installed Chromium in an isolated headless profile")
	}
	t.Setenv("HOME", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<title>OwnCode test</title><input id="name" oninput="window.entered=this.value"><button id="go" onclick="document.querySelector('#out').innerText=window.entered">Show</button><p id="out"></p><script>document.addEventListener("keydown",e=>document.querySelector("#out").innerText=e.key)</script>`))
	}))
	defer server.Close()
	browser := NewBrowser(func() BrowserOptions { return BrowserOptions{Mode: "embedded"} })
	defer browser.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	for _, r := range []Request{{Action: "open", URL: server.URL}, {Action: "type", Selector: "#name", Text: "automation works"}, {Action: "click", Selector: "#go"}} {
		r.Session = "test"
		result, err := browser.Run(ctx, r)
		if err != nil {
			t.Fatal(err)
		}
		if r.Action == "click" && !strings.Contains(result.Text, "automation works") {
			t.Fatalf("click observation: %s", result.Text)
		}
	}
	pressed, err := browser.Run(ctx, Request{Session: "test", Action: "press", Key: "ArrowDown"})
	if err != nil || !strings.Contains(pressed.Text, "ArrowDown") {
		t.Fatalf("arrow key observation = %s, error = %v", pressed.Text, err)
	}
	exerciseInteractions(t, ctx, browser, "test")
	result, err := browser.Run(ctx, Request{Session: "test", Action: "screenshot"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Image) == 0 {
		t.Fatal("missing screenshot")
	}
	_, err = browser.Run(ctx, Request{Session: "test", Action: "close"})
	if err != nil {
		t.Fatal(err)
	}
	if len(browser.sessions) != 0 {
		t.Fatal("closed session retained")
	}
}
func TestURLValidation(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "javascript:alert(1)", "https://user:password@example.com", "http://"} {
		if ValidateURL(raw) == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	if err := ValidateURL("http://localhost:3000"); err != nil {
		t.Fatal(err)
	}
}
func TestBrowserQueueCancellation(t *testing.T) {
	b := NewBrowser(func() BrowserOptions { return BrowserOptions{Mode: "embedded"} })
	s, err := b.session("test")
	if err != nil {
		t.Fatal(err)
	}
	s.gate <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = b.Run(ctx, Request{Session: "test", Action: "observe"}); err == nil {
		t.Fatal("queued action ignored cancellation")
	}
	<-s.gate
	if err = b.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = b.session("new"); err == nil {
		t.Fatal("closed browser accepted a session")
	}
}
func TestFindBrowserOverride(t *testing.T) {
	if _, err := FindBrowser("embedded", "relative-path"); err == nil {
		t.Fatal("relative executable accepted")
	}
}
