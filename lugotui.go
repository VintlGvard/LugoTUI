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
)

var (
	ErrPanicRecovered    = tui.ErrPanicRecovered
	NewProgram           = tui.NewProgram
	RunApp               = tui.RunApp
	RunSource            = tui.RunSource
	QuickRun             = tui.QuickRun
	Run                  = tui.Run
	RunOptions           = tui.RunOptions
	Load                 = tui.Load
	LoadNamed            = tui.LoadNamed
	DefaultTheme         = tui.DefaultTheme
	WithAppFile          = tui.WithAppFile
	WithSource           = tui.WithSource
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
)

func NewOSC52Clipboard(w io.Writer) *OSC52Clipboard {
	return tui.NewOSC52Clipboard(w)
}

func NewScreenRenderer(fn func(screen tcell.Screen, rt *Runtime, diagnostic string)) Renderer {
	return tui.RendererFunc(fn)
}
