package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/chromedp/cdproto/page"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"unicode/utf8"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// BrowserOptions defines launch settings copied when a session first opens.
type BrowserOptions struct{ Mode, Executable, CDPURL string }
type browserTab struct {
	ctx    context.Context
	cancel context.CancelFunc
}
type browserSession struct {
	tabs    map[string]browserTab
	active  string
	next    int
	gate    chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	options BrowserOptions
	retired bool
}

// Browser owns long-lived CDP sessions. Close cancels their lifetime contexts.
type Browser struct {
	mu       sync.Mutex
	sessions map[string]*browserSession
	options  func() BrowserOptions
	closed   bool
}

// NewBrowser creates a browser manager without launching any processes.
func NewBrowser(options func() BrowserOptions) *Browser {
	return &Browser{sessions: make(map[string]*browserSession), options: options}
}
func (b *Browser) session(id string) (*browserSession, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, fmt.Errorf("browser is closed")
	}
	if s := b.sessions[id]; s != nil {
		return s, nil
	}
	if len(b.sessions) >= 8 {
		return nil, fmt.Errorf("browser session limit reached; close an unused session")
	}
	s := &browserSession{gate: make(chan struct{}, 1), options: b.options()}
	b.sessions[id] = s
	return s, nil
}

// Run serializes actions within the requesting agent session.
func (b *Browser) Run(ctx context.Context, r Request) (Result, error) {
	if r.Action == "open" || r.Action == "navigate" || r.Action == "new_tab" {
		if err := ValidateURL(r.URL); err != nil {
			return Result{}, err
		}
	}
	if r.Action == "switch_tab" && r.Tab == "" {
		return Result{}, fmt.Errorf("switch_tab requires tab")
	}
	s, err := b.session(r.Session)
	if err != nil {
		return Result{}, err
	}
	select {
	case s.gate <- struct{}{}:
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
	defer func() { <-s.gate }()
	b.mu.Lock()
	closed := b.closed
	b.mu.Unlock()
	if closed || s.retired {
		return Result{}, fmt.Errorf("browser session was closed; open a new session")
	}
	if r.Action == "close" {
		s.retired = true
		if s.cancel != nil {
			s.cancel()
			s.cancel = nil
			s.ctx = nil
		}
		b.mu.Lock()
		delete(b.sessions, r.Session)
		b.mu.Unlock()
		return Result{Text: "Browser session closed"}, nil
	}
	if s.ctx == nil {
		if err = b.start(ctx, s); err != nil {
			s.retired = true
			b.mu.Lock()
			delete(b.sessions, r.Session)
			b.mu.Unlock()
			return Result{}, err
		}
	}
	if s.tabs == nil {
		s.tabs = make(map[string]browserTab)
	}
	if r.Action == "tabs" {
		ids := make([]string, 0, len(s.tabs))
		for id := range s.tabs {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		encoded, _ := json.Marshal(map[string]any{"tabs": ids, "active": s.active})
		return Result{Text: string(encoded)}, nil
	}
	if r.Tab != "" {
		if _, ok := s.tabs[r.Tab]; !ok {
			return Result{}, fmt.Errorf("unknown tab; use tabs")
		}
		s.active = r.Tab
	}
	if r.Action == "close_tab" {
		if tab, ok := s.tabs[s.active]; ok {
			tab.cancel()
			delete(s.tabs, s.active)
		}
		s.active = ""
		for id := range s.tabs {
			s.active = id
			break
		}
		return Result{Text: "Tab closed"}, nil
	}
	if r.Action == "new_tab" || s.active == "" {
		if len(s.tabs) >= 8 {
			return Result{}, fmt.Errorf("tab limit reached; close a tab")
		}
		tabCtx, tabCancel := chromedp.NewContext(s.ctx)
		stopStart := context.AfterFunc(ctx, tabCancel)
		err := chromedp.Run(tabCtx)
		stopped := stopStart()
		if err != nil || !stopped || ctx.Err() != nil {
			tabCancel()
			if err != nil {
				return Result{}, err
			}
			return Result{}, ctx.Err()
		}
		s.next++
		s.active = fmt.Sprintf("tab-%d", s.next)
		s.tabs[s.active] = browserTab{tabCtx, tabCancel}
	}
	callCtx, cancel := context.WithCancel(s.tabs[s.active].ctx)
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	defer cancel()
	var tasks chromedp.Tasks
	selector := r.Selector
	if r.Element > 0 {
		selector = fmt.Sprintf("[data-owncode-ref='%d']", r.Element)
	}
	switch r.Action {
	case "open", "navigate", "new_tab":
		if err := ValidateURL(r.URL); err != nil {
			return Result{}, err
		}
		tasks = append(tasks, chromedp.Navigate(r.URL))
	case "observe", "switch_tab":
	case "back":
		tasks = append(tasks, chromedp.NavigateBack())
	case "forward":
		tasks = append(tasks, chromedp.NavigateForward())
	case "reload":
		tasks = append(tasks, chromedp.Reload())
	case "dialog":
		err := chromedp.Run(callCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			return page.HandleJavaScriptDialog(r.Accept).WithPromptText(r.Text).Do(ctx)
		}))
		return Result{Text: "Dialog handled"}, err
	case "wait":
		if selector == "" {
			return Result{}, fmt.Errorf("selector or element is required")
		}
		tasks = append(tasks, chromedp.WaitVisible(selector, chromedp.ByQuery))
	case "hover", "double_click", "right_click", "drag":
		if selector == "" {
			return Result{}, fmt.Errorf("selector or element is required")
		}
		tasks = append(tasks, browserPointer(r, selector))
	case "upload":
		if selector == "" {
			return Result{}, fmt.Errorf("selector or element is required")
		}
		if err := ValidateFiles(r.Files); err != nil {
			return Result{}, err
		}
		tasks = append(tasks, chromedp.SetUploadFiles(selector, r.Files, chromedp.ByQuery))
	case "click":
		if selector == "" {
			return Result{}, fmt.Errorf("selector or element is required")
		}
		tasks = append(tasks, chromedp.Click(selector, chromedp.ByQuery))
	case "type", "select":
		if selector == "" {
			return Result{}, fmt.Errorf("selector or element is required")
		}
		encodedSelector, _ := json.Marshal(selector)
		encodedText, _ := json.Marshal(r.Text)
		tasks = append(tasks, chromedp.Evaluate(fmt.Sprintf(fillJS, encodedSelector, encodedText), nil))
	case "press":
		key, ok := browserKey(r.Key)
		if !ok {
			return Result{}, fmt.Errorf("unsupported key; use a named key or single character")
		}
		if err := validateModifiers(r.Modifiers); err != nil {
			return Result{}, err
		}
		var modifiers []input.Modifier
		for _, m := range r.Modifiers {
			modifiers = append(modifiers, map[string]input.Modifier{"Meta": input.ModifierMeta, "Alt": input.ModifierAlt, "Control": input.ModifierCtrl, "Shift": input.ModifierShift}[m])
		}
		tasks = append(tasks, chromedp.KeyEvent(key, chromedp.KeyModifiers(modifiers...)))
	case "scroll":
		tasks = append(tasks, chromedp.ActionFunc(func(ctx context.Context) error {
			return input.DispatchMouseEvent(input.MouseWheel, 0, 0).WithDeltaX(float64(r.X)).WithDeltaY(float64(r.Y)).Do(ctx)
		}))
	case "screenshot":
		var data []byte
		if err = chromedp.Run(callCtx, chromedp.CaptureScreenshot(&data)); err != nil {
			return Result{}, err
		}
		path, err := artifact(data)
		return Result{Text: "Screenshot: " + path, Image: data}, err
	default:
		return Result{}, fmt.Errorf("unsupported browser action %q", r.Action)
	}
	var observation json.RawMessage
	tasks = append(tasks, chromedp.Evaluate(observeJS, &observation))
	if err = chromedp.Run(callCtx, tasks); err != nil {
		return Result{}, err
	}
	return Result{Text: bounded(string(observation))}, nil
}
func (b *Browser) start(ctx context.Context, s *browserSession) error {
	var alloc context.Context
	var allocCancel context.CancelFunc
	if s.options.Mode == "cdp" {
		if err := ValidateURL(s.options.CDPURL); err != nil {
			return fmt.Errorf("configure automation.cdpURL with an HTTP discovery endpoint: %w", err)
		}
		alloc, allocCancel = chromedp.NewRemoteAllocator(context.Background(), s.options.CDPURL)
	} else {
		executable, err := FindBrowser(s.options.Mode, s.options.Executable)
		if err != nil {
			return err
		}
		options := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
		options = append(options, chromedp.ExecPath(executable), chromedp.Flag("headless", s.options.Mode == "embedded"), chromedp.WindowSize(1280, 800))
		// Temporary profiles isolate concurrent workers and never copy personal cookies.
		alloc, allocCancel = chromedp.NewExecAllocator(context.Background(), options...)
	}
	browser, cancel := chromedp.NewContext(alloc)
	stop := context.AfterFunc(ctx, func() { cancel(); allocCancel() })
	err := chromedp.Run(browser)
	stopped := stop()
	if err != nil || !stopped || ctx.Err() != nil {
		cancel()
		allocCancel()
		if err != nil {
			return err
		}
		return ctx.Err()
	}
	s.ctx = browser
	s.cancel = func() { cancel(); allocCancel() }
	return nil
}

// Close releases all owned tabs, profiles, and browser processes.
func (b *Browser) Close() error {
	b.mu.Lock()
	b.closed = true
	all := b.sessions
	b.sessions = make(map[string]*browserSession)
	b.mu.Unlock()
	for _, s := range all {
		s.gate <- struct{}{}
		s.retired = true
		if s.cancel != nil {
			s.cancel()
		}
		<-s.gate
	}
	return nil
}

// FindBrowser finds an installed browser without downloading a runtime.
func FindBrowser(mode, override string) (string, error) {
	if override != "" {
		if !filepath.IsAbs(override) {
			return "", fmt.Errorf("browser executable must be an absolute path")
		}
		if info, err := os.Stat(override); err == nil && !info.IsDir() {
			return override, nil
		}
		return "", fmt.Errorf("browser executable not found")
	}
	names := []string{"google-chrome", "chromium", "chromium-browser", "brave-browser", "chrome"}
	if mode == "chrome" {
		names = []string{"google-chrome", "google-chrome-stable", "chrome"}
	}
	if mode == "brave" {
		names = []string{"brave-browser", "brave"}
	}
	if runtime.GOOS == "darwin" {
		names = []string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser", "/Applications/Chromium.app/Contents/MacOS/Chromium"}
		if mode == "brave" {
			names = names[1:2]
		}
		if mode == "chrome" {
			names = names[:1]
		}
	}
	if runtime.GOOS == "windows" {
		for _, base := range []string{os.Getenv("PROGRAMFILES"), os.Getenv("LOCALAPPDATA"), os.Getenv("PROGRAMFILES(X86)")} {
			if base == "" {
				continue
			}
			if mode != "brave" {
				names = append(names, filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"))
			}
			if mode != "chrome" {
				names = append(names, filepath.Join(base, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"))
			}
		}
	}
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no installed browser found; install Chrome or Brave, or set automation.executable")
}
func browserKey(key string) (string, bool) {
	keys := map[string]string{"Enter": "\r", "Tab": "\t", "Escape": "\u001b", "Backspace": "\b", "ArrowUp": kb.ArrowUp, "ArrowDown": kb.ArrowDown, "ArrowLeft": kb.ArrowLeft, "ArrowRight": kb.ArrowRight, "Home": kb.Home, "End": kb.End, "PageUp": kb.PageUp, "PageDown": kb.PageDown, "Delete": kb.Delete, "Space": " ", "F1": kb.F1, "F2": kb.F2, "F3": kb.F3, "F4": kb.F4, "F5": kb.F5, "F6": kb.F6, "F7": kb.F7, "F8": kb.F8, "F9": kb.F9, "F10": kb.F10, "F11": kb.F11, "F12": kb.F12}
	if utf8.RuneCountInString(key) == 1 {
		return key, true
	}
	v, ok := keys[key]
	return v, ok
}

// Inspection is bounded before it crosses the CDP boundary. Element refs expire on page changes.
const observeJS = `(()=>{document.querySelectorAll("[data-owncode-ref]").forEach(e=>e.removeAttribute("data-owncode-ref"));let elements=[...document.querySelectorAll('a,button,input,textarea,select,[role="button"],[contenteditable="true"]')].filter(e=>e.getClientRects().length).slice(0,120);return {url:location.href,title:document.title,text:document.body?.innerText.slice(0,10000),elements:elements.map((e,i)=>{e.setAttribute('data-owncode-ref',String(i+1));return {element:i+1,tag:e.tagName,label:(e.getAttribute('aria-label')||e.innerText||e.getAttribute('placeholder')||'').slice(0,160),type:e.getAttribute('type')}})}})()`

const fillJS = `(()=>{const e=document.querySelector(%s),text=%s;if(!e)throw Error('Element missing; observe again');if(e.disabled||e.readOnly)throw Error('Element cannot be edited');e.focus();if(e.isContentEditable){e.textContent=text;}else{const proto=e instanceof HTMLTextAreaElement?HTMLTextAreaElement.prototype:e instanceof HTMLSelectElement?HTMLSelectElement.prototype:HTMLInputElement.prototype;const setter=Object.getOwnPropertyDescriptor(proto,'value')?.set;if(!setter)throw Error('Element cannot be filled');setter.call(e,text);}e.dispatchEvent(new Event('input',{bubbles:true}));e.dispatchEvent(new Event('change',{bubbles:true}));return true})()`
