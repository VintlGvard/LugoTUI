//go:build !linux

package runner

import "errors"

func runPlatformSandbox(_ Options) error {
	return errors.New("process sandbox requires Linux")
}
