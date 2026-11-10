package engine

import (
	"encoding/base64"
	"fmt"
	"io"
	"sync"

	"github.com/gdamore/tcell/v2"
)

type Renderer interface {
	Draw(screen tcell.Screen, rt *Runtime, diagnostic string)
}

type RendererFunc func(screen tcell.Screen, rt *Runtime, diagnostic string)

func (f RendererFunc) Draw(screen tcell.Screen, rt *Runtime, diagnostic string) {
	f(screen, rt, diagnostic)
}

type Clipboard interface {
	Read() (string, error)
	Write(text string) error
}

type OSC52Clipboard struct {
	mu   sync.RWMutex
	text string
	out  io.Writer
}

func NewOSC52Clipboard(out io.Writer) *OSC52Clipboard {
	return &OSC52Clipboard{out: out}
}

func (c *OSC52Clipboard) Read() (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.text, nil
}

func (c *OSC52Clipboard) Write(text string) error {
	if len(text) > maxText {
		return fmt.Errorf("clipboard text exceeds 1 MiB")
	}
	c.mu.Lock()
	c.text = text
	out := c.out
	c.mu.Unlock()
	if out != nil {
		encoded := base64.StdEncoding.EncodeToString([]byte(text))
		_, _ = fmt.Fprintf(out, "\x1b]52;c;%s\x07", encoded)
	}
	return nil
}

type StateObserver func(key string, oldValue, newValue any)

func (r *Runtime) ClipboardAdapter() Clipboard {
	if r.Clipboard == nil {
		r.Clipboard = NewOSC52Clipboard(nil)
	}
	return r.Clipboard
}

func (r *Runtime) notifyState(key string, oldVal, newVal any) {
	for _, fn := range r.StateObservers {
		if fn != nil {
			fn(key, oldVal, newVal)
		}
	}
}
