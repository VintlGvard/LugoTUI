//go:build !windows

package engine

import (
	"errors"
	"net"
	"os"
)

func listenAgent(path string) (net.Listener, error) {
	if _, e := os.Lstat(path); e == nil {
		return nil, errors.New("socket path already exists; choose unused path")
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	listener, e := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if e != nil {
		return nil, e
	}
	if e = os.Chmod(path, 0600); e != nil {
		listener.Close()
		return nil, e
	}
	return listener, nil
}
