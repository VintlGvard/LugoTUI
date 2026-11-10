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
	return runPlatformSandbox(options)
}
