// Package lugotui provides a flat root entry point for LugoTUI, mirroring
// the tui package so callers can import either github.com/VintlGvard/LugoTUI
// or github.com/VintlGvard/LugoTUI/tui.
package lugotui

import (
	"io"

	"github.com/VintlGvard/LugoTUI/tui"
	"github.com/gdamore/tcell/v2"
)

type (
	Program         = tui.Program
	Option          = tui.Option
	Options         = tui.Options
	Runtime         = tui.Runtime
	ViewFunc        = tui.ViewFunc
	HostEvent       = tui.HostEvent
	EventHandler    = tui.EventHandler
	EventMiddleware = tui.EventMiddleware
	ActionHandler   = tui.ActionHandler
	InputFilter     = tui.InputFilter
	StateObserver   = tui.StateObserver
	Renderer        = tui.Renderer
	RendererFunc    = tui.RendererFunc
	Clipboard       = tui.Clipboard
	OSC52Clipboard  = tui.OSC52Clipboard
	Node            = tui.Node
	Rect            = tui.Rect
	WidgetState     = tui.WidgetState
	TaskResult      = tui.TaskResult
	Theme           = tui.Theme
	TableColumn     = tui.TableColumn
	TreeItem        = tui.TreeItem
	FileEntry       = tui.FileEntry
	HelpBinding     = tui.HelpBinding
	HighlightSpan   = tui.HighlightSpan
	BorderGlyphs    = tui.BorderGlyphs
	Canvas          = tui.Canvas
)

var (
	BorderSingle  = tui.BorderSingle
	BorderRounded = tui.BorderRounded
	BorderDouble  = tui.BorderDouble
	BorderThick   = tui.BorderThick
	BorderASCII   = tui.BorderASCII
)

var (
	ErrPanicRecovered    = tui.ErrPanicRecovered
	ErrQuit              = tui.ErrQuit
	NewProgram           = tui.NewProgram
	RunApp               = tui.RunApp
	RunSource            = tui.RunSource
	QuickRun             = tui.QuickRun
	QuickView            = tui.QuickView
	QuickViewFunc        = tui.QuickViewFunc
	Run                  = tui.Run
	RunOptions           = tui.RunOptions
	Load                 = tui.Load
	LoadNamed            = tui.LoadNamed
	LoadView             = tui.LoadView
	LoadViewFunc         = tui.LoadViewFunc
	DefaultTheme         = tui.DefaultTheme
	WithAppFile          = tui.WithAppFile
	WithSource           = tui.WithSource
	WithView             = tui.WithView
	WithViewFunc         = tui.WithViewFunc
	WithInitialData      = tui.WithInitialData
	WithState            = tui.WithState
	WithAction           = tui.WithAction
	WithSocket           = tui.WithSocket
	WithSandbox          = tui.WithSandbox
	WithCheck            = tui.WithCheck
	WithSnapshot         = tui.WithSnapshot
	WithReservedKeys     = tui.WithReservedKeys
	WithTheme            = tui.WithTheme
	WithOnEvent          = tui.WithOnEvent
	WithHandlers         = tui.WithHandlers
	WithMiddleware       = tui.WithMiddleware
	WithInputFilter      = tui.WithInputFilter
	WithStateObserver    = tui.WithStateObserver
	WithRenderer         = tui.WithRenderer
	WithRendererFunc     = tui.WithRendererFunc
	WithClipboard        = tui.WithClipboard
	WithScreen           = tui.WithScreen
	WithAltScreen        = tui.WithAltScreen
	WithInline           = tui.WithInline
	WithMouse            = tui.WithMouse
	WithPaste            = tui.WithPaste
	WithFPS              = tui.WithFPS
	WithoutPanicRecovery = tui.WithoutPanicRecovery
	WithOnReady          = tui.WithOnReady

	NewNode    = tui.NewNode
	Column     = tui.Column
	Row        = tui.Row
	Box        = tui.Box
	Overlay    = tui.Overlay
	Spacer     = tui.Spacer
	Panel      = tui.Panel
	Scroll     = tui.Scroll
	Modal      = tui.Modal
	Text       = tui.Text
	Textf      = tui.Textf
	Button     = tui.Button
	Input      = tui.Input
	Textarea   = tui.Textarea
	List       = tui.List
	Table      = tui.Table
	Tree       = tui.Tree
	Viewport   = tui.Viewport
	Filepicker = tui.Filepicker
	Spinner    = tui.Spinner
	Progress   = tui.Progress
	Paginator  = tui.Paginator
	Timer      = tui.Timer
	Stopwatch  = tui.Stopwatch
	Help       = tui.Help
	Cursor     = tui.Cursor
	Custom     = tui.Custom
	If         = tui.If
	IfElse     = tui.IfElse
)

func Map[T any](items []T, fn func(index int, item T) *Node) []*Node {
	return tui.Map(items, fn)
}

func NewOSC52Clipboard(w io.Writer) *OSC52Clipboard {
	return tui.NewOSC52Clipboard(w)
}

func NewScreenRenderer(fn func(screen tcell.Screen, rt *Runtime, diagnostic string)) Renderer {
	return tui.RendererFunc(fn)
}
