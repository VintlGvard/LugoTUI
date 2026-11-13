package text

import (
	"slices"
	"strings"
	"testing"
)

func TestIsASCIIAndTextWidth(t *testing.T) {
	if !IsASCII("") || !IsASCII("hello ~!") {
		t.Fatal("expected printable ASCII strings to return true")
	}
	for _, s := range []string{"\n", "\t", "\x7f", "привет", "👩‍💻"} {
		if IsASCII(s) {
			t.Fatalf("expected IsASCII(%q) to be false", s)
		}
	}

	for _, tc := range []struct {
		in   string
		want int
	}{
		{"", 0},
		{"ascii", 5},
		{"e\u0301", 1},
		{"界", 2},
		{"👩‍💻", 2},
		{"hi界🙂", 6},
	} {
		if got := TextWidth(tc.in); got != tc.want {
			t.Errorf("TextWidth(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}

	if allocs := testing.AllocsPerRun(20, func() { _ = TextWidth("fast ascii path") }); allocs != 0 {
		t.Fatalf("expected 0 allocs on ASCII TextWidth, got %g", allocs)
	}
}

func TestGraphemesAndTruncate(t *testing.T) {
	if got := Graphemes(""); len(got) != 0 {
		t.Fatalf("expected empty graphemes, got %q", got)
	}
	wantG := []string{"e\u0301", "👩‍💻", "界", "a"}
	if got := Graphemes("e\u0301👩‍💻界a"); !slices.Equal(got, wantG) {
		t.Fatalf("Graphemes = %q, want %q", got, wantG)
	}

	for _, tc := range []struct {
		in       string
		maxWidth int
		want     string
	}{
		{"", 5, ""},
		{"hello", 0, ""},
		{"hello", -2, ""},
		{"hello", 3, "hel"},
		{"hello", 5, "hello"},
		{"hello", 10, "hello"},
		{"e\u0301b", 1, "e\u0301"},
		{"👩‍💻a", 1, ""},
		{"👩‍💻a", 2, "👩‍💻"},
		{"a界b", 2, "a"},
		{"a界b", 3, "a界"},
	} {
		if got := Truncate(tc.in, tc.maxWidth); got != tc.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tc.in, tc.maxWidth, got, tc.want)
		}
	}
}

func TestLinesLimitedLinesAndWrapLinesOffsets(t *testing.T) {
	if got := Lines("hello world again", 11, true); !slices.Equal(got, []string{"hello world", "again"}) {
		t.Fatalf("Lines wrap mismatch: %q", got)
	}
	if got := Lines("  hello", 6, true); !slices.Equal(got, []string{"  hell", "o"}) {
		t.Fatalf("leading spaces wrap: %q", got)
	}
	if got := Lines("ab ", 2, true); !slices.Equal(got, []string{"ab"}) {
		t.Fatalf("trailing space wrap: %q", got)
	}
	if got := Lines("a\nb", 2, false); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("nowrap mismatch: %q", got)
	}
	if got := LimitedLines("a\nb\nc", 0, true, 2); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("zero-width limited lines mismatch: %q", got)
	}
	if got := WrapLines("abc", 2, true, -1); len(got) != 0 {
		t.Fatalf("negative limit should return empty slice, got %+v", got)
	}

	for _, src := range []string{
		"",
		"ab",
		"a b",
		"ab cd",
		"x   \n  \ny",
		"  leading   spaces  \nnext",
		"👩‍💻界 test\nпривет мир",
		"longwordwithoutspaces",
		"a\n\nb\n",
	} {
		for _, wrap := range []bool{false, true} {
			for width := 0; width <= 8; width++ {
				all := WrapLines(src, width, wrap, int(^uint(0)>>1))
				prevStart := 0
				for i, wl := range all {
					if wl.StartByte < 0 || wl.EndByte < wl.StartByte || wl.EndByte > len(src) {
						t.Fatalf("src=%q w=%d wrap=%v line=%d invalid bounds [%d,%d) len=%d",
							src, width, wrap, i, wl.StartByte, wl.EndByte, len(src))
					}
					if wl.StartByte < prevStart {
						t.Fatalf("src=%q w=%d wrap=%v line=%d non-monotonic StartByte %d < %d",
							src, width, wrap, i, wl.StartByte, prevStart)
					}
					prevStart = wl.StartByte
					if got := src[wl.StartByte:wl.EndByte]; got != wl.Text {
						t.Fatalf("src=%q w=%d wrap=%v line=%d slice %q != Text %q",
							src, width, wrap, i, got, wl.Text)
					}
				}
				for limit := 0; limit <= len(all)+1; limit++ {
					got := LimitedLines(src, width, wrap, limit)
					want := Lines(src, width, wrap)[:min(limit, len(all))]
					if !slices.Equal(got, want) {
						t.Fatalf("src=%q w=%d wrap=%v limit=%d: got %q, want %q",
							src, width, wrap, limit, got, want)
					}
				}
			}
		}
	}
}

func BenchmarkWrapLinesASCII(b *testing.B) {
	src := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 10)
	b.ReportAllocs()
	for b.Loop() {
		_ = WrapLines(src, 40, true, 64)
	}
}

func BenchmarkWrapLinesUnicode(b *testing.B) {
	src := strings.Repeat("Привет мир 👩‍💻界 тест переноса строк. ", 10)
	b.ReportAllocs()
	for b.Loop() {
		_ = WrapLines(src, 24, true, 64)
	}
}
