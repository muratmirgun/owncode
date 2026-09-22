package automation

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestComputerQueueCanCancel(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS queue")
	}
	computer := NewComputer()
	defer computer.Close()
	computer.gate <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := computer.Run(ctx, Request{Action: "observe", App: "Finder"}); err == nil {
		t.Fatal("queued action ignored cancellation")
	}
	<-computer.gate
}
func TestComputerShutdownCancelsQueuedAction(t *testing.T) {
	computer := NewComputer()
	computer.gate <- struct{}{}
	done := make(chan error, 1)
	go func() { _, err := computer.Run(context.Background(), Request{Action: "apps"}); done <- err }()
	_ = computer.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("closed backend accepted work")
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel queued work")
	}
	<-computer.gate
}

func TestComputerReadOnlyIntegration(t *testing.T) {
	if os.Getenv("OWNCODE_COMPUTER_TEST") != "1" || runtime.GOOS != "darwin" {
		t.Skip("opt-in native macOS inspection")
	}
	computer := NewComputer()
	defer computer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := computer.Run(ctx, Request{Action: "apps"})
	if err != nil {
		t.Fatal(err)
	}
	var applications []string
	if err = json.Unmarshal([]byte(result.Text), &applications); err != nil {
		t.Fatal(err)
	}
	if len(applications) == 0 {
		t.Fatal("native inspection returned no applications")
	}
}

func TestComputerInputValidation(t *testing.T) {
	for _, r := range []Request{{Action: "drag", EndX: 1001}, {Action: "move", X: -1}, {Action: "scroll", Y: 2001}, {Action: "press", Modifiers: []string{"injected"}}} {
		if validateComputerRequest(r) == nil {
			t.Errorf("accepted invalid request: %+v", r)
		}
	}
	if err := validateComputerRequest(Request{Action: "drag", X: 100, Y: 200, EndX: 800, EndY: 900}); err != nil {
		t.Fatal(err)
	}
}

// Construct native events without posting input or accessing personal applications.
func TestComputerNativeEventConstruction(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS event bridge")
	}
	script := strings.ReplaceAll(computerJS, "Application('System Events')", `({applicationProcesses:{byName:()=>({exists:()=>true,frontmost:()=>true})}})`)
	script = strings.ReplaceAll(script, "$.AXIsProcessTrusted()", "true")
	script = strings.ReplaceAll(script, "$.CGEventPost(0,e)", "void(0)")
	for _, action := range []string{"click_at", "right_click", "double_click", "move", "drag", "scroll"} {
		t.Run(action, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			data, _ := json.Marshal(Request{Action: action, App: "Fixture", X: 100, Y: 200, EndX: 800, EndY: 900})
			if _, err := runCommand(ctx, "/usr/bin/osascript", "-l", "JavaScript", "-e", script, string(data)); err != nil {
				t.Fatal(err)
			}
		})
	}
}
