package automation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestExtensionIntegration(t *testing.T) {
	if os.Getenv("OWNCODE_EXTENSION_TEST") != "1" {
		t.Skip("set OWNCODE_EXTENSION_TEST=1 for isolated Brave extension validation")
	}
	t.Setenv("HOME", t.TempDir())
	executable, err := FindBrowser("brave", "")
	if err != nil {
		t.Fatal(err)
	}
	extension, err := filepath.Abs("../../browser-extension")
	if err != nil {
		t.Fatal(err)
	}
	options := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	options = append(options, chromedp.ExecPath(executable), chromedp.Flag("disable-extensions", false), chromedp.Flag("disable-extensions-except", extension), chromedp.Flag("load-extension", extension))
	ctx, deadline := context.WithTimeout(context.Background(), 35*time.Second)
	defer deadline()
	alloc, cancelAlloc := chromedp.NewExecAllocator(ctx, options...)
	defer cancelAlloc()
	browser, cancelBrowser := chromedp.NewContext(alloc)
	defer cancelBrowser()
	if err = chromedp.Run(browser); err != nil {
		t.Fatal(err)
	}
	var extensionID string
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for extensionID == "" {
		targets, err := chromedp.Targets(browser)
		if err != nil {
			t.Fatal(err)
		}
		for _, target := range targets {
			if strings.HasPrefix(target.URL, "chrome-extension://") && strings.HasSuffix(target.URL, "/background.js") {
				extensionID = strings.Split(strings.TrimPrefix(target.URL, "chrome-extension://"), "/")[0]
			}
		}
		if extensionID != "" {
			break
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("extension worker did not start")
		}
	}
	relay := NewRelay()
	if err = relay.start(); err != nil {
		t.Fatal(err)
	}
	defer relay.Close()
	pairing, _ := json.Marshal(map[string]string{"url": "http://" + relay.address, "token": relay.token})
	if err = chromedp.Run(browser, chromedp.Navigate("chrome-extension://"+extensionID+"/popup.html"), chromedp.SetValue("#pairing", string(pairing), chromedp.ByQuery), chromedp.Click("#connect", chromedp.ByQuery)); err != nil {
		t.Fatal(err)
	}
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><title>Relay fixture</title><input id="name" oninput="document.querySelector('#out').textContent=this.value"><p id="out">Ready</p></html>`))
	}))
	defer fixture.Close()
	if _, err = relay.Run(ctx, Request{Action: "open", URL: fixture.URL, Session: "integration"}); err != nil {
		t.Fatal(err)
	}
	for {
		result, err := relay.Run(ctx, Request{Action: "observe", Session: "integration"})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(result.Text, "Relay fixture") {
			break
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("fixture did not load")
		}
	}
	result, err := relay.Run(ctx, Request{Action: "type", Selector: "#name", Text: "relay works", Session: "integration"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Text, "relay works") {
		t.Fatal("extension did not deliver input events")
	}
	exerciseInteractions(t, ctx, relay, "integration")
	if _, err = relay.Run(ctx, Request{Action: "close", Session: "integration"}); err != nil {
		t.Fatal(err)
	}
}
