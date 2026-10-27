package tui_test

import (
	"context"
	"fmt"
	"log"

	"github.com/VintlGvard/LugoTUI/tui"
)

func ExampleRunOptions() {
	if err := tui.RunOptions(tui.Options{AppFile: "app.lua"}); err != nil {
		log.Fatal(err)
	}
}

func ExampleRuntime_StartTask() {
	r, err := tui.Load(`local ui = require('ui')
return {
  api_version = 1,
  view = ui.text {id = 'result', text = 'Waiting'},
  actions = {done = function(e) ui.patch('result', {text = e.text}) end}
}`)
	if err != nil {
		panic(err)
	}
	defer r.Close()

	_, err = r.StartTask("done", func(ctx context.Context) (map[string]any, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return map[string]any{"text": "Ready"}, nil
	})
	if err != nil {
		panic(err)
	}
	if err := r.HandleTask(<-r.TaskResults()); err != nil {
		panic(err)
	}
	fmt.Println(r.Node("result").Props["text"])
	// Output: Ready
}
