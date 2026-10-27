package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"strings"
	"time"

	"github.com/VintlGvard/LugoTUI/tui"
)

func handler() func(*tui.Runtime, tui.HostEvent) error {
	var owner *tui.Runtime
	var task uint64
	return func(r *tui.Runtime, event tui.HostEvent) error {
		if owner != r {
			owner, task = r, 0
		}
		switch event.Action {
		case "cancel":
			r.CancelTask(task)
			task = 0
			return r.Action("cancelled", nil)
		case "run":
			payload, ok := event.Payload.(map[string]any)
			if !ok {
				return errors.New("run payload must be an object")
			}
			text, ok := payload["text"].(string)
			if !ok || len(text) > 4096 {
				return errors.New("run text must be a string of at most 4096 bytes")
			}
			r.CancelTask(task)
			var err error
			task, err = r.StartTask("finished", func(ctx context.Context) (map[string]any, error) {
				timer := time.NewTimer(time.Second)
				defer timer.Stop()
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-timer.C:
				}
				if text == "fail" {
					return map[string]any{"error": "controlled failure"}, nil
				}
				return map[string]any{"text": strings.ToUpper(text)}, nil
			})
			return err
		default:
			return errors.New("unknown host event: " + event.Action)
		}
	}
}

func main() {
	app := flag.String("app", "examples/worker/app.lua", "Lua application path (relative to working directory)")
	check := flag.Bool("check", false, "validate without terminal")
	snapshot := flag.Bool("snapshot", false, "print headless snapshot")
	flag.Parse()
	if err := tui.RunOptions(tui.Options{AppFile: *app, Check: *check, Snapshot: *snapshot, OnEvent: handler()}); err != nil {
		log.Fatal(err)
	}
}
