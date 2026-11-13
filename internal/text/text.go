package text

import (
	"bytes"
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
	g := uniseg.NewGraphemes(s)
	out := []string{}
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
		current := ""
		curStart := lineStart
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
			if w+gw > width && current != "" {
				if strings.TrimSpace(g) == "" {
					trimmed := strings.TrimRight(current, " \t")
					out = append(out, WrappedLine{Text: trimmed, StartByte: curStart, EndByte: curStart + len(trimmed)})
					current = ""
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
					current = rem
					w = TextWidth(current)
					if w+gw > width && current != "" {
						if len(out) >= limit {
							return out
						}
						out = append(out, WrappedLine{Text: current, StartByte: curStart, EndByte: curStart + len(current)})
						current = ""
						w = 0
					}
				} else {
					out = append(out, WrappedLine{Text: current, StartByte: curStart, EndByte: curStart + len(current)})
					current = ""
					w = 0
				}
				if strings.TrimSpace(g) == "" && current == "" {
					continue
				}
			}
			if current == "" {
				curStart = pos
			}
			current += g
			w += gw
		}
		if len(out) >= limit {
			return out
		}
		if !wrappedOnSpace || current != "" || len(line) == 0 {
			if current == "" {
				curStart = lineEnd
			}
			out = append(out, WrappedLine{Text: current, StartByte: curStart, EndByte: curStart + len(current)})
		}
		if nl < 0 {
			break
		}
		lineStart = lineEnd + 1
	}
	return out
}

func wrapASCII(line string, lineStart, width, limit int, out []WrappedLine) []WrappedLine {
	if len(line) == 0 {
		return append(out, WrappedLine{Text: "", StartByte: lineStart, EndByte: lineStart})
	}
	buf := make([]byte, 0, width+16)
	bufStart := lineStart
	for i := 0; i < len(line); i++ {
		if len(out) >= limit {
			return out
		}
		b := line[i]
		if len(buf)+1 > width && len(buf) > 0 {
			if b == ' ' || b == '\t' {
				trimmed := bytes.TrimRight(buf, " \t")
				out = append(out, WrappedLine{Text: string(trimmed), StartByte: bufStart, EndByte: bufStart + len(trimmed)})
				buf = buf[:0]
				for i+1 < len(line) && (line[i+1] == ' ' || line[i+1] == '\t') {
					i++
				}
				if i+1 >= len(line) {
					return out
				}
				continue
			}
			cut := bytes.LastIndexAny(buf, " \t")
			trimmed := bytes.TrimRight(buf[:max(0, cut)], " \t")
			if cut > 0 && len(trimmed) > 0 {
				out = append(out, WrappedLine{Text: string(trimmed), StartByte: bufStart, EndByte: bufStart + len(trimmed)})
				rem := bytes.TrimLeft(buf[cut+1:], " \t")
				remStart := bufStart + len(buf) - len(rem)
				buf = append(buf[:0], rem...)
				bufStart = remStart
				if len(buf)+1 > width && len(buf) > 0 {
					if len(out) >= limit {
						return out
					}
					out = append(out, WrappedLine{Text: string(buf), StartByte: bufStart, EndByte: bufStart + len(buf)})
					buf = buf[:0]
				}
			} else {
				out = append(out, WrappedLine{Text: string(buf), StartByte: bufStart, EndByte: bufStart + len(buf)})
				buf = buf[:0]
			}
			if (b == ' ' || b == '\t') && len(buf) == 0 {
				continue
			}
		}
		if len(buf) == 0 {
			bufStart = lineStart + i
		}
		buf = append(buf, b)
	}
	if len(out) >= limit {
		return out
	}
	if len(buf) == 0 {
		end := lineStart + len(line)
		return append(out, WrappedLine{Text: "", StartByte: end, EndByte: end})
	}
	return append(out, WrappedLine{Text: string(buf), StartByte: bufStart, EndByte: bufStart + len(buf)})
}
