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
