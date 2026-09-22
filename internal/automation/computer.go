package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// Computer serializes desktop actions across every agent. Waiting remains cancellable.
type Computer struct {
	gate     chan struct{}
	lifetime context.Context
	cancel   context.CancelFunc
}

// NewComputer creates a shared desktop queue with an explicit lifetime.
func NewComputer() *Computer {
	ctx, cancel := context.WithCancel(context.Background())
	return &Computer{gate: make(chan struct{}, 1), lifetime: ctx, cancel: cancel}
}

// Close cancels queued and active desktop commands.
func (c *Computer) Close() error { c.cancel(); return nil }

// Run executes a native action without interpolating model input into code.
func (c *Computer) Run(ctx context.Context, r Request) (Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(c.lifetime, cancel)
	defer stop()
	if c.lifetime.Err() != nil {
		return Result{}, fmt.Errorf("computer backend closed")
	}

	if err := validateComputerRequest(r); err != nil {
		return Result{}, err
	}
	if runtime.GOOS != "darwin" {
		return Result{}, fmt.Errorf("the macos backend requires macOS")
	}
	select {
	case c.gate <- struct{}{}:
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
	defer func() { <-c.gate }()
	if r.Action == "screenshot" {
		f, err := os.CreateTemp("", "owncode-screen-*.png")
		if err != nil {
			return Result{}, err
		}
		name := f.Name()
		_ = f.Close()
		defer os.Remove(name)
		if _, err = runCommand(ctx, "/usr/sbin/screencapture", "-x", "-m", name); err != nil {
			return Result{}, fmt.Errorf("screen capture failed; allow Screen Recording for your terminal: %w", err)
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return Result{}, err
		}
		if len(data) > 12<<20 {
			return Result{}, fmt.Errorf("screenshot exceeds 12 MiB")
		}
		path, err := artifact(data)
		return Result{Text: "Primary display screenshot. Pointer coordinates use 0..1000 across this image, independent of pixel size. Desktop screenshot: " + path, Image: data}, err
	}
	payload, err := json.Marshal(r)
	if err != nil {
		return Result{}, err
	}
	output, err := runCommand(ctx, "/usr/bin/osascript", "-l", "JavaScript", "-e", computerJS, string(payload))
	if err != nil {
		if r.Action == "drag" || r.Action == "click_at" || r.Action == "double_click" || r.Action == "right_click" {
			cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
			_, releaseErr := runCommand(cleanup, "/usr/bin/osascript", "-l", "JavaScript", "-e", releasePointerJS)
			stop()
			if releaseErr != nil {
				err = fmt.Errorf("%w; pointer release failed: %v", err, releaseErr)
			}
		}
		return Result{}, fmt.Errorf("desktop action failed; check macOS Accessibility and Automation permissions for your terminal: %w", err)
	}
	return Result{Text: bounded(output)}, nil
}

type cappedOutput struct{ bytes.Buffer }

func (b *cappedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if remain := 65536 - b.Len(); remain > 0 {
		_, _ = b.Buffer.Write(p[:min(remain, n)])
	}
	return n, nil
}
func runCommand(ctx context.Context, program string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.WaitDelay = time.Second
	var output, stderr cappedOutput
	cmd.Stdout = &output
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, bounded(stderr.String()))
	}
	return output.String(), nil
}

// Arguments travel as JSON, never executable AppleScript. Clicks require a unique exact accessible name.
const computerJS = `function run(argv){
 const r=JSON.parse(argv[0]), se=Application('System Events');
 if(r.action==='apps')return JSON.stringify(se.applicationProcesses.whose({backgroundOnly:false}).name());
 if(!r.app)throw Error('app is required; inspect apps first');
 const p=se.applicationProcesses.byName(r.app);
 if(!p.exists())throw Error('application is not running');
 if(r.action==='activate'){p.frontmost=true;return 'Activated '+r.app;}
 if(['click_at','double_click','right_click','move','drag','scroll'].includes(r.action)){
  if(!p.frontmost())throw Error('activate the target app before pointer input');
  ObjC.import('CoreGraphics');ObjC.import('ApplicationServices');
  if(!$.AXIsProcessTrusted())throw Error('Accessibility permission is required');
  const b=$.CGDisplayBounds($.CGMainDisplayID());
  const point=(x,y)=>$.CGPointMake(b.origin.x+x*(b.size.width-1)/1000,b.origin.y+y*(b.size.height-1)/1000);
  const button=r.action==='right_click'?1:0, down=button?3:1, up=button?4:2;
  function event(type,x,y,count){const e=$.CGEventCreateMouseEvent(null,type,point(x,y),button);if(count)$.CGEventSetIntegerValueField(e,1,count);$.CGEventPost(0,e);}
  if(r.action==='scroll'){
   const e=$.CGEventCreateScrollWheelEvent(null,0,1,-(r.y||0));
   $.CGEventSetIntegerValueField(e,$.kCGScrollWheelEventDeltaAxis2,-(r.x||0));
   $.CGEventSetIntegerValueField(e,$.kCGScrollWheelEventPointDeltaAxis2,-(r.x||0));
   $.CGEventPost(0,e);return 'Scrolled';
  }
  if(r.action==='move'){event(5,r.x||0,r.y||0);return 'Pointer moved';}
  try{
   event(down,r.x||0,r.y||0,1);
   if(r.action==='drag')for(let i=1;i<=20;i++)event(6,(r.x||0)+(r.endX-(r.x||0))*i/20,(r.y||0)+(r.endY-(r.y||0))*i/20);
  }finally{event(up,r.action==='drag'?r.endX:r.x||0,r.action==='drag'?r.endY:r.y||0,1);}
  if(r.action==='double_click'){event(down,r.x||0,r.y||0,2);event(up,r.x||0,r.y||0,2);}
  return 'Pointer action completed';
 }
 if(r.action==='type'||r.action==='press'){
  if(!p.frontmost())throw Error('activate the target app before keyboard input');
  if(r.action==='type'){se.keystroke(r.text||'');return 'Text entered';}
  const keys={Enter:36,Tab:48,Escape:53,Backspace:51,Delete:117,Home:115,End:119,PageUp:116,PageDown:121,Space:49,ArrowUp:126,ArrowDown:125,ArrowLeft:123,ArrowRight:124,F1:122,F2:120,F3:99,F4:118,F5:96,F6:97,F7:98,F8:100,F9:101,F10:109,F11:103,F12:111};
  const names={Meta:'command down',Control:'control down',Alt:'option down',Shift:'shift down'};
  const options={using:(r.modifiers||[]).map(m=>names[m])};
  if(r.key in keys)se.keyCode(keys[r.key],options);
  else if(r.key&&r.key.length===1)se.keystroke(r.key,options);
  else throw Error('unsupported key');return 'Key sent';
 }
 let items=[],nodes=[],visited=0;
 function walk(e,depth){if(depth>7||visited++>=400)return;let name='',role='';try{name=e.name()||'';role=e.role()||'';}catch(_){}
  let position=null,size=null;try{position=e.position();size=e.size();}catch(_){};if(name||role){items.push({element:items.length+1,name:name.slice(0,160),role,position,size});nodes.push(e);}
  let children=[];try{children=e.uiElements();}catch(_){};for(const child of children)walk(child,depth+1);
 }
 for(const w of p.windows())walk(w,0);
 if(r.action==='observe')return JSON.stringify({app:r.app,elements:items,truncated:visited>=400});
 if(r.action==='click'){
  if(r.element){if(r.element<1||r.element>nodes.length)throw Error('Element reference expired; observe again');nodes[r.element-1].actions.byName('AXPress').perform();return 'Clicked element '+r.element;}
  if(!r.selector)throw Error('selector must be an exact accessible name from observe');
  let matches=[];for(let i=0;i<items.length;i++)if(items[i].name===r.selector)matches.push(nodes[i]);
  if(matches.length!==1)throw Error('accessible name is absent or ambiguous; observe again');
  matches[0].actions.byName('AXPress').perform();return 'Clicked '+r.selector;
 }
 throw Error('unsupported desktop action');
}`

// A killed osascript cannot run its finally block. Release held buttons separately.
const releasePointerJS = `ObjC.import('CoreGraphics');const source=$.CGEventCreate(null),p=$.CGEventGetLocation(source);for(const [kind,button] of [[2,0],[4,1]]){const e=$.CGEventCreateMouseEvent(null,kind,p,button);$.CGEventPost(0,e);}`

func validateComputerRequest(r Request) error {
	if err := validateModifiers(r.Modifiers); err != nil {
		return err
	}
	switch r.Action {
	case "click_at", "double_click", "right_click", "move", "drag":
		for _, n := range []int{r.X, r.Y, r.EndX, r.EndY} {
			if n < 0 || n > 1000 {
				return fmt.Errorf("pointer coordinates must be between 0 and 1000")
			}
		}
	case "scroll":
		if r.X < -2000 || r.X > 2000 || r.Y < -2000 || r.Y > 2000 {
			return fmt.Errorf("scroll deltas must be between -2000 and 2000")
		}
	}
	return nil
}

func validateModifiers(modifiers []string) error {
	for _, m := range modifiers {
		switch m {
		case "Meta", "Control", "Alt", "Shift":
		default:
			return fmt.Errorf("unsupported modifier %q", m)
		}
	}
	return nil
}
