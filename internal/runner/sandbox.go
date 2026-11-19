package runner

import (
	"errors"

	"github.com/VintlGvard/LugoTUI/internal/engine"
)

func Sandbox(options Options) error {
	if options.OnEvent != nil || options.ReservedKeys != nil || options.Theme != (engine.Theme{}) {
		return errors.New("sandbox cannot serialize OnEvent, ReservedKeys, or Theme options")
	}
	if options.Renderer != nil || options.InputFilter != nil || len(options.Middlewares) > 0 || len(options.StateObservers) > 0 || options.Clipboard != nil {
		return errors.New("sandbox cannot serialize custom hooks, middlewares, or interfaces")
	}
	if options.Source != "" || options.View != nil || options.ViewFunc != nil || len(options.InitialData) > 0 || len(options.GoActions) > 0 || options.AllowHostActions || options.Inline || options.InlineHeight != 0 || options.AltScreen != nil || options.Mouse != nil || options.BracketedPaste != nil || options.FPS != 0 || options.Screen != nil || options.OnReady != nil || options.DisablePanicRecovery {
		return errors.New("sandbox cannot serialize inline source, views, state, Go actions, or terminal options")
	}
	return runPlatformSandbox(options)
}
