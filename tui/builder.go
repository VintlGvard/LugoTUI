package tui

import "github.com/VintlGvard/LugoTUI/internal/engine"

type TableColumn = engine.TableColumn

type TreeItem = engine.TreeItem

type FileEntry = engine.FileEntry

type HelpBinding = engine.HelpBinding

type HighlightSpan = engine.HighlightSpan

func NewNode(kind string, children ...*Node) *Node {
	return engine.NewNode(kind, children...)
}

func Column(children ...*Node) *Node  { return engine.Column(children...) }
func Row(children ...*Node) *Node     { return engine.Row(children...) }
func Box(children ...*Node) *Node     { return engine.Box(children...) }
func Overlay(children ...*Node) *Node { return engine.Overlay(children...) }
func Spacer() *Node                   { return engine.Spacer() }

func Panel(title string, children ...*Node) *Node {
	return engine.Panel(title, children...)
}

func Scroll(children ...*Node) *Node {
	return engine.Scroll(children...)
}

func Modal(id, title string, children ...*Node) *Node {
	return engine.Modal(id, title, children...)
}

func Text(text string) *Node {
	return engine.Text(text)
}

func Textf(format string, args ...any) *Node {
	return engine.Textf(format, args...)
}

func Button(label string, onClick ...string) *Node {
	return engine.Button(label, onClick...)
}

func Input(bindKey string, placeholder ...string) *Node {
	return engine.Input(bindKey, placeholder...)
}

func Textarea(bindKey string, placeholder ...string) *Node {
	return engine.Textarea(bindKey, placeholder...)
}

func List(items ...string) *Node {
	return engine.List(items...)
}

func Table(columns []TableColumn, rows [][]string) *Node {
	return engine.Table(columns, rows)
}

func Tree(items ...TreeItem) *Node {
	return engine.Tree(items...)
}

func Viewport(text string, highlights ...HighlightSpan) *Node {
	return engine.Viewport(text, highlights...)
}

func Filepicker(entries ...FileEntry) *Node {
	return engine.Filepicker(entries...)
}

func Spinner(preset ...string) *Node {
	return engine.Spinner(preset...)
}

func Progress(percent float64) *Node {
	return engine.Progress(percent)
}

func Paginator(page, perPage, total int) *Node {
	return engine.Paginator(page, perPage, total)
}

func Timer(durationMs int, onTimeout ...string) *Node {
	return engine.Timer(durationMs, onTimeout...)
}

func Stopwatch() *Node {
	return engine.Stopwatch()
}

func Help(bindings ...HelpBinding) *Node {
	return engine.Help(bindings...)
}

func Cursor() *Node {
	return engine.Cursor()
}

func If(cond bool, node *Node) *Node {
	return engine.If(cond, node)
}

func IfElse(cond bool, thenNode, elseNode *Node) *Node {
	return engine.IfElse(cond, thenNode, elseNode)
}

func Map[T any](items []T, fn func(index int, item T) *Node) []*Node {
	return engine.Map(items, fn)
}
