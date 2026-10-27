package engine

import (
	"github.com/gdamore/tcell/v2"
	"testing"
)

func TestScrollExtent(t *testing.T) {
	for _, tc := range []struct {
		name, children string
		want           int
	}{
		{"fits", `ui.text{text='one',height=1,shrink=0}`, 0},
		{"overflows", `ui.text{text='one',height=15,shrink=0}`, 5},
		{"gap and margins", `ui.text{text='one',height=6,margin=1,shrink=0},ui.text{text='two',height=6,margin=1,shrink=0}`, 8},
		{"hidden", `ui.text{text='one',height=1,shrink=0},ui.text{text='hidden',height=100,hidden=true}`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Load(`return {view=ui.scroll{id='scroll',gap=2,` + tc.children + `}}`)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatal(err)
			}
			defer screen.Fini()
			screen.SetSize(30, 10)
			r.State["scroll"].Scroll = 100
			r.Draw(screen, "")
			if got := r.State["scroll"].Scroll; got != tc.want {
				t.Fatalf("scroll=%d, want %d", got, tc.want)
			}
		})
	}
}
