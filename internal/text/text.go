package text

import (
	"strings"

	"github.com/rivo/uniseg"
)

type WrappedLine struct {
	Text      string
	StartByte int
	EndByte   int
}

func IsASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] >= 0x7f {
			return false
		}
	}
	return true
}

func TextWidth(s string) int {
	if IsASCII(s) {
		return len(s)
	}
	return uniseg.StringWidth(s)
}

func Graphemes(s string) []string {
	if s == "" {
		return []string{}
	}
	if IsASCII(s) {
		out := make([]string, len(s))
		for i := 0; i < len(s); i++ {
			out[i] = s[i : i+1]
		}
		return out
	}
	g := uniseg.NewGraphemes(s)
	out := make([]string, 0, len(s)/2+1)
	for g.Next() {
		out = append(out, g.Str())
	}
	return out
}

func Truncate(s string, maxWidth int) string {
	if maxWidth <= 0 || s == "" {
		return ""
	}
	if IsASCII(s) {
		if len(s) <= maxWidth {
			return s
		}
		return s[:maxWidth]
	}
	w := 0
	endByte := 0
	g := uniseg.NewGraphemes(s)
	for g.Next() {
		gw := g.Width()
		if w+gw > maxWidth {
			break
		}
		w += gw
		endByte += len(g.Str())
	}
	return s[:endByte]
}

func Lines(text string, width int, wrap bool) []string {
	return LimitedLines(text, width, wrap, int(^uint(0)>>1))
}

func LimitedLines(text string, width int, wrap bool, limit int) []string {
	spans := WrapLines(text, width, wrap, limit)
	out := make([]string, len(spans))
	for i, s := range spans {
		out[i] = s.Text
	}
	return out
}

func WrapLines(text string, width int, wrap bool, limit int) []WrappedLine {
	out := []WrappedLine{}
	if limit <= 0 {
		return out
	}
	for lineStart := 0; ; {
		if len(out) >= limit {
			return out
		}
		nl := strings.IndexByte(text[lineStart:], '\n')
		lineEnd := len(text)
		if nl >= 0 {
			lineEnd = lineStart + nl
		}
		line := text[lineStart:lineEnd]
		if !wrap || width <= 0 {
			out = append(out, WrappedLine{Text: line, StartByte: lineStart, EndByte: lineEnd})
			if nl < 0 {
				break
			}
			lineStart = lineEnd + 1
			continue
		}
		if IsASCII(line) {
			out = wrapASCII(line, lineStart, width, limit, out)
			if nl < 0 {
				break
			}
			lineStart = lineEnd + 1
			continue
		}
		curStart, curEnd := lineStart, lineStart
		bytePos := lineStart
		w := 0
		wrappedOnSpace := false
		iterator := uniseg.NewGraphemes(line)
		for iterator.Next() {
			if len(out) >= limit {
				return out
			}
			g := iterator.Str()
			pos := bytePos
			bytePos += len(g)
			gw := iterator.Width()
			if wrappedOnSpace {
				if strings.TrimSpace(g) == "" {
					continue
				}
				wrappedOnSpace = false
			}
			if w+gw > width && curStart < curEnd {
				current := text[curStart:curEnd]
				if strings.TrimSpace(g) == "" {
					trimmed := strings.TrimRight(current, " \t")
					out = append(out, WrappedLine{Text: trimmed, StartByte: curStart, EndByte: curStart + len(trimmed)})
					curStart, curEnd = 0, 0
					w = 0
					wrappedOnSpace = true
					continue
				}
				cut := strings.LastIndexAny(current, " \t")
				trimmed := strings.TrimRight(current[:max(0, cut)], " \t")
				if cut > 0 && len(trimmed) > 0 {
					out = append(out, WrappedLine{Text: trimmed, StartByte: curStart, EndByte: curStart + len(trimmed)})
					rem := strings.TrimLeft(current[cut+1:], " \t")
					curStart = curStart + len(current) - len(rem)
					w = TextWidth(rem)
					if w+gw > width && curStart < curEnd {
						if len(out) >= limit {
							return out
						}
						out = append(out, WrappedLine{Text: text[curStart:curEnd], StartByte: curStart, EndByte: curEnd})
						curStart, curEnd = 0, 0
						w = 0
					}
				} else {
					out = append(out, WrappedLine{Text: current, StartByte: curStart, EndByte: curEnd})
					curStart, curEnd = 0, 0
					w = 0
				}
				if strings.TrimSpace(g) == "" && curStart == curEnd {
					continue
				}
			}
			if curStart == curEnd {
				curStart = pos
			}
			curEnd = bytePos
			w += gw
		}
		if len(out) >= limit {
			return out
		}
		if !wrappedOnSpace || curStart < curEnd || len(line) == 0 {
			if curStart == curEnd {
				curStart, curEnd = lineEnd, lineEnd
			}
			out = append(out, WrappedLine{Text: text[curStart:curEnd], StartByte: curStart, EndByte: curEnd})
		}
		if nl < 0 {
			break
		}
		lineStart = lineEnd + 1
	}
	return out
}

func trimRightSpaces(s string, start, end int) int {
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return end
}

func trimLeftSpaces(s string, start, end int) int {
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	return start
}

func wrapASCII(line string, lineStart, width, limit int, out []WrappedLine) []WrappedLine {
	if len(line) == 0 {
		return append(out, WrappedLine{Text: "", StartByte: lineStart, EndByte: lineStart})
	}
	start, end := 0, 0
	for i := 0; i < len(line); i++ {
		if len(out) >= limit {
			return out
		}
		b := line[i]
		if end-start+1 > width && end > start {
			if b == ' ' || b == '\t' {
				trimEnd := trimRightSpaces(line, start, end)
				out = append(out, WrappedLine{Text: line[start:trimEnd], StartByte: lineStart + start, EndByte: lineStart + trimEnd})
				start, end = 0, 0
				for i+1 < len(line) && (line[i+1] == ' ' || line[i+1] == '\t') {
					i++
				}
				if i+1 >= len(line) {
					return out
				}
				continue
			}
			cut := strings.LastIndexAny(line[start:end], " \t")
			trimEnd := start
			if cut > 0 {
				trimEnd = trimRightSpaces(line, start, start+cut)
			}
			if cut > 0 && trimEnd > start {
				out = append(out, WrappedLine{Text: line[start:trimEnd], StartByte: lineStart + start, EndByte: lineStart + trimEnd})
				start = trimLeftSpaces(line, start+cut+1, end)
				if end-start+1 > width && end > start {
					if len(out) >= limit {
						return out
					}
					out = append(out, WrappedLine{Text: line[start:end], StartByte: lineStart + start, EndByte: lineStart + end})
					start, end = 0, 0
				}
			} else {
				out = append(out, WrappedLine{Text: line[start:end], StartByte: lineStart + start, EndByte: lineStart + end})
				start, end = 0, 0
			}
			if (b == ' ' || b == '\t') && start == end {
				continue
			}
		}
		if start == end {
			start = i
		}
		end = i + 1
	}
	if len(out) >= limit {
		return out
	}
	if start == end {
		endPos := lineStart + len(line)
		return append(out, WrappedLine{Text: "", StartByte: endPos, EndByte: endPos})
	}
	return append(out, WrappedLine{Text: line[start:end], StartByte: lineStart + start, EndByte: lineStart + end})
}
