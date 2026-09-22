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
)

const interactionFixture = `<title>Interaction fixture</title><input id="name"><input id="file" type="file" onchange="out.textContent=this.files[0].name"><select id="choice" onchange="out.textContent=this.value"><option value="one">One</option><option value="two">Two</option></select><button id="target" onmouseenter="out.textContent='hovered'" ondblclick="out.textContent='double clicked'" oncontextmenu="event.preventDefault();out.textContent='right clicked'">Target</button><div id="start" style="width:100px;height:40px;background:gray" onpointerdown="window.started=true">Drag start</div><div id="end" style="width:100px;height:40px;background:blue" onpointerup="if(window.started)out.textContent='dragged'">Drag end</div><p id="out"></p><script>document.addEventListener('keydown',e=>{if(e.ctrlKey&&e.key==='k'){e.preventDefault();out.textContent='shortcut'}})</script>`

func exerciseInteractions(t *testing.T, ctx context.Context, backend Backend, session string) {
	t.Helper()
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(interactionFixture))
	}))
	defer fixture.Close()
	run := func(r Request) Result {
		t.Helper()
		r.Session = session
		result, err := backend.Run(ctx, r)
		if err != nil {
			t.Fatalf("%s: %v", r.Action, err)
		}
		return result
	}
	run(Request{Action: "open", URL: fixture.URL})
	run(Request{Action: "wait", Selector: "#target"})
	for _, tc := range []struct {
		request Request
		want    string
	}{
		{Request{Action: "select", Selector: "#choice", Text: "two"}, "two"},
		{Request{Action: "hover", Selector: "#target"}, "hovered"},
		{Request{Action: "double_click", Selector: "#target"}, "double clicked"},
		{Request{Action: "right_click", Selector: "#target"}, "right clicked"},
		{Request{Action: "drag", Selector: "#start", TargetSelector: "#end"}, "dragged"},
		{Request{Action: "press", Key: "k", Modifiers: []string{"Control"}}, "shortcut"},
	} {
		if result := run(tc.request); !strings.Contains(result.Text, tc.want) {
			t.Fatalf("%s missing %q: %s", tc.request.Action, tc.want, result.Text)
		}
	}
	file := filepath.Join(t.TempDir(), "fixture-upload.txt")
	if err := os.WriteFile(file, []byte("upload fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if result := run(Request{Action: "upload", Selector: "#file", Files: []string{file}}); !strings.Contains(result.Text, "fixture-upload.txt") {
		t.Fatal("file upload did not emit change")
	}
	var first struct {
		Active any   `json:"active"`
		Tabs   []any `json:"tabs"`
	}
	if err := json.Unmarshal([]byte(run(Request{Action: "tabs"}).Text), &first); err != nil {
		t.Fatal(err)
	}
	run(Request{Action: "new_tab", URL: fixture.URL + "/second"})
	var second struct {
		Tabs []any `json:"tabs"`
	}
	if err := json.Unmarshal([]byte(run(Request{Action: "tabs"}).Text), &second); err != nil {
		t.Fatal(err)
	}
	if len(second.Tabs) != len(first.Tabs)+1 {
		t.Fatal("new tab was not tracked")
	}
	run(Request{Action: "close_tab"})
	run(Request{Action: "reload"})
	run(Request{Action: "wait", Selector: "#target"})
	if result := run(Request{Action: "observe"}); !strings.Contains(result.Text, "Interaction fixture") {
		t.Fatal("remaining tab lost")
	}
}

func TestUploadValidation(t *testing.T) {
	for _, files := range [][]string{nil, {"relative"}, {t.TempDir()}, {"/nonexistent-owncode-upload"}} {
		if ValidateFiles(files) == nil {
			t.Errorf("accepted %v", files)
		}
	}
}
