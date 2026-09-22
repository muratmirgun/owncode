package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
)

// ValidateFiles checks explicit local file paths before a browser upload.
func ValidateFiles(files []string) error {
	if len(files) == 0 || len(files) > 16 {
		return fmt.Errorf("upload requires between 1 and 16 files")
	}
	for _, path := range files {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("upload paths must be absolute")
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("upload file: %w", err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("upload requires regular files")
		}
	}
	return nil
}

type browserPoint struct{ X, Y float64 }

func browserPointer(r Request, selector string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) (err error) {
		point := func(selector string, scroll bool) (browserPoint, error) {
			var p browserPoint
			encoded, _ := json.Marshal(selector)
			err := chromedp.Evaluate(fmt.Sprintf(`(()=>{const e=document.querySelector(%s);if(!e)throw Error('Element missing');if(%t)e.scrollIntoView({block:'center',inline:'center'});const b=e.getBoundingClientRect();if(!b.width||!b.height||b.x+b.width/2<0||b.y+b.height/2<0||b.x+b.width/2>=innerWidth||b.y+b.height/2>=innerHeight)throw Error('Element must be visible in the viewport');return {X:b.x+b.width/2,Y:b.y+b.height/2}})()`, encoded, scroll), &p).Do(ctx)
			return p, err
		}
		from, err := point(selector, true)
		if err != nil {
			return err
		}
		to := from
		if r.Action == "drag" {
			if r.TargetSelector == "" {
				return fmt.Errorf("drag requires targetSelector")
			}
			to, err = point(r.TargetSelector, true)
			if err != nil {
				return err
			}
			from, err = point(selector, true)
			if err != nil {
				return err
			}
			to, err = point(r.TargetSelector, false)
			if err != nil {
				return err
			}
		}
		if err = input.DispatchMouseEvent(input.MouseMoved, from.X, from.Y).Do(ctx); err != nil {
			return err
		}
		if r.Action == "hover" {
			return nil
		}
		button := input.Left
		if r.Action == "right_click" {
			button = input.Right
		}
		pressed := false
		defer func() {
			if !pressed {
				return
			}
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			defer cancel()
			err = errors.Join(err, input.DispatchMouseEvent(input.MouseReleased, to.X, to.Y).WithButton(button).WithClickCount(1).Do(cleanup))
		}()
		count := int64(1)
		if r.Action == "double_click" {
			count = 2
		}
		for n := int64(1); n <= count; n++ {
			pressed = true
			if err = input.DispatchMouseEvent(input.MousePressed, from.X, from.Y).WithButton(button).WithClickCount(n).Do(ctx); err != nil {
				return err
			}
			if r.Action == "drag" {
				for step := 1; step <= 20; step++ {
					f := float64(step) / 20
					if err = input.DispatchMouseEvent(input.MouseMoved, from.X+(to.X-from.X)*f, from.Y+(to.Y-from.Y)*f).WithButton(button).WithButtons(1).Do(ctx); err != nil {
						return err
					}
				}
			}
			if err = input.DispatchMouseEvent(input.MouseReleased, to.X, to.Y).WithButton(button).WithClickCount(n).Do(ctx); err != nil {
				return err
			}
			pressed = false
		}
		return nil
	})
}
