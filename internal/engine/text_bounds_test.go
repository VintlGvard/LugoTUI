package engine

import (
	"slices"
	"strings"
	"testing"
)

func TestLimitedLinesNeverExceedsLimit(t *testing.T) {
	for _, text := range []string{"ab", "a b", "ab cd", "👩‍💻界", "ab\ncd"} {
		for width := 1; width <= 4; width++ {
			all := Lines(text, width, true)
			for limit := 0; limit <= len(all)+1; limit++ {
				got := LimitedLines(text, width, true, limit)
				want := all[:min(limit, len(all))]
				if !slices.Equal(got, want) {
					t.Errorf("text %q width %d limit %d: got %q, want %q", text, width, limit, got, want)
				}
			}
		}
	}
}

func TestTextareaSuggestionDoesNotScanText(t *testing.T) {
	n := &Node{Type: "textarea"}
	s := &WidgetState{Value: strings.Repeat("hello world\n", 10000)}
	allocations := testing.AllocsPerRun(10, func() {
		if got := editorSuggestion(n, s); got != "" {
			t.Fatalf("textarea suggestion: %q", got)
		}
	})
	if allocations != 0 {
		t.Fatalf("textarea suggestions allocated %g times", allocations)
	}
}

func TestNearestPositionPrefersClosestRow(t *testing.T) {
	positions := [][2]int{{0, 0}, {200001, 1}}
	if got := nearestPosition(positions, 200001, 0); got != 0 {
		t.Fatalf("cursor jumped to another row: %d", got)
	}
	if got := nearestPosition(positions, 0, 1); got != 1 {
		t.Fatalf("cursor stayed on another row: %d", got)
	}
	if got := nearestPosition([][2]int{{0, 0}, {4, 0}}, 3, 0); got != 1 {
		t.Fatalf("nearest column: %d", got)
	}
	if got := nearestPosition([][2]int{{0, 0}, {4, 0}}, 2, 0); got != 0 {
		t.Fatalf("equal distances must keep first position: %d", got)
	}
}

func TestLimitedLinesLeadingAndTrailingSpacesAndPasteSanitization(t *testing.T) {
	if got := Lines("  hello", 6, true); !slices.Equal(got, []string{"  hell", "o"}) {
		t.Fatalf("leading spaces wrap: got %q, want [\"  hell\" \"o\"]", got)
	}
	if got := Lines("ab ", 2, true); !slices.Equal(got, []string{"ab"}) {
		t.Fatalf("trailing space wrap: got %q, want [\"ab\"]", got)
	}
	r, err := Load(`return {
		view = {
			type = 'column',
			id = 'root',
			{ type = 'input', id = 'in', width = 10, height = 1 },
			{ type = 'textarea', id = 'ta', width = 10, height = 4 },
		}
	}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.SetFocus("in"); err != nil {
		t.Fatal(err)
	}
	if err := r.Paste("a\tb\x00c"); err != nil {
		t.Fatal(err)
	}
	if got := r.State["in"].Value; got != "a bc" {
		t.Fatalf("input paste sanitization: got %q, want %q", got, "a bc")
	}
	if err := r.SetFocus("ta"); err != nil {
		t.Fatal(err)
	}
	if err := r.Paste("line1\r\nline2\t\x00end"); err != nil {
		t.Fatal(err)
	}
	if got := r.State["ta"].Value; got != "line1\nline2 end" {
		t.Fatalf("textarea paste sanitization: got %q, want %q", got, "line1\nline2 end")
	}
}
