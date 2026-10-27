package tui_test

import (
	"testing"

	"github.com/VintlGvard/LugoTUI/tui"
	"github.com/gdamore/tcell/v2"
)

func BenchmarkInitLoad(b *testing.B) {
	demo := tui.Demo()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, err := tui.Load(demo)
		if err != nil {
			b.Fatal(err)
		}
		r.Close()
	}
}

func BenchmarkLayoutPass(b *testing.B) {
	r, err := tui.Load(tui.Demo())
	if err != nil {
		b.Fatal(err)
	}
	defer r.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := r.Layout(120, 40); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRenderDiffPass(b *testing.B) {
	r, err := tui.Load(tui.Demo())
	if err != nil {
		b.Fatal(err)
	}
	defer r.Close()

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		b.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(120, 40)
	r.Draw(s, "")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Draw(s, "")
	}
}

func BenchmarkStateMutation(b *testing.B) {
	r, err := tui.Load(tui.Demo())
	if err != nil {
		b.Fatal(err)
	}
	defer r.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := r.Action("notes_changed", nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStateMutationPatch(b *testing.B) {
	r, err := tui.Load(tui.Demo())
	if err != nil {
		b.Fatal(err)
	}
	defer r.Close()

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		b.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(120, 40)
	r.Draw(s, "")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := r.Action("notes_changed", nil); err != nil {
			b.Fatal(err)
		}
		r.Draw(s, "")
	}
}
