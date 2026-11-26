//go:build windows

package engine

import (
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

func isNamedPipePath(path string) bool {
	return strings.HasPrefix(strings.ToLower(path), `\\.\pipe\`)
}

func listenAgent(path string) (net.Listener, error) {
	if isNamedPipePath(path) {
		return listenNamedPipe(path)
	}
	if _, e := os.Lstat(path); e == nil {
		return nil, errors.New("socket path already exists; choose unused path")
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	return net.Listen("unix", path)
}

type pipeAddr string

func (a pipeAddr) Network() string { return "pipe" }
func (a pipeAddr) String() string  { return string(a) }

type winPipeListener struct {
	path       string
	namePtr    *uint16
	closeEvent windows.Handle
	once       sync.Once
	mu         sync.Mutex
	closed     bool
	firstPipe  windows.Handle
}

func listenNamedPipe(path string) (net.Listener, error) {
	namePtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	closeEvent, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return nil, err
	}
	first, err := createPipeInstance(namePtr, windows.FILE_FLAG_FIRST_PIPE_INSTANCE)
	if err != nil {
		windows.CloseHandle(closeEvent)
		return nil, err
	}
	return &winPipeListener{
		path:       path,
		namePtr:    namePtr,
		closeEvent: closeEvent,
		firstPipe:  first,
	}, nil
}

func createPipeInstance(namePtr *uint16, extraFlags uint32) (windows.Handle, error) {
	return windows.CreateNamedPipe(
		namePtr,
		windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_OVERLAPPED|extraFlags,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT,
		windows.PIPE_UNLIMITED_INSTANCES,
		4096,
		4096,
		0,
		nil,
	)
}

func (l *winPipeListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil, net.ErrClosed
	}
	h := l.firstPipe
	l.firstPipe = 0
	l.mu.Unlock()

	if h == 0 {
		var err error
		h, err = createPipeInstance(l.namePtr, 0)
		if err != nil {
			return nil, err
		}
	}

	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		windows.CloseHandle(h)
		return nil, err
	}
	defer windows.CloseHandle(ev)

	var ol windows.Overlapped
	ol.HEvent = ev
	err = windows.ConnectNamedPipe(h, &ol)
	switch err {
	case nil, windows.ERROR_PIPE_CONNECTED:
		return &winPipeConn{h: h, addr: pipeAddr(l.path)}, nil
	case windows.ERROR_IO_PENDING:
		handles := []windows.Handle{ev, l.closeEvent}
		wait, waitErr := windows.WaitForMultipleObjects(handles, false, windows.INFINITE)
		if waitErr != nil || wait != windows.WAIT_OBJECT_0 {
			windows.CancelIoEx(h, &ol)
			var n uint32
			_ = windows.GetOverlappedResult(h, &ol, &n, true)
			windows.CloseHandle(h)
			if waitErr != nil {
				return nil, waitErr
			}
			return nil, net.ErrClosed
		}
		var n uint32
		if resErr := windows.GetOverlappedResult(h, &ol, &n, false); resErr != nil {
			windows.CloseHandle(h)
			return nil, resErr
		}
		return &winPipeConn{h: h, addr: pipeAddr(l.path)}, nil
	default:
		windows.CloseHandle(h)
		return nil, err
	}
}

func (l *winPipeListener) Close() error {
	l.once.Do(func() {
		l.mu.Lock()
		l.closed = true
		first := l.firstPipe
		l.firstPipe = 0
		l.mu.Unlock()
		if first != 0 {
			windows.CloseHandle(first)
		}
		_ = windows.SetEvent(l.closeEvent)
		_ = windows.CloseHandle(l.closeEvent)
	})
	return nil
}

func (l *winPipeListener) Addr() net.Addr {
	return pipeAddr(l.path)
}

type winPipeConn struct {
	h             windows.Handle
	addr          pipeAddr
	once          sync.Once
	mu            sync.Mutex
	readDeadline  time.Time
	writeDeadline time.Time
}

func (c *winPipeConn) Read(p []byte) (int, error) {
	c.mu.Lock()
	deadline := c.readDeadline
	c.mu.Unlock()
	n, err := c.doOverlapped(deadline, func(ol *windows.Overlapped) error {
		return windows.ReadFile(c.h, p, nil, ol)
	})
	if errors.Is(err, windows.ERROR_BROKEN_PIPE) || errors.Is(err, windows.ERROR_NO_DATA) {
		return n, io.EOF
	}
	return n, err
}

func (c *winPipeConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	deadline := c.writeDeadline
	c.mu.Unlock()
	return c.doOverlapped(deadline, func(ol *windows.Overlapped) error {
		return windows.WriteFile(c.h, p, nil, ol)
	})
}

func (c *winPipeConn) doOverlapped(deadline time.Time, op func(*windows.Overlapped) error) (int, error) {
	timeoutMs := uint32(windows.INFINITE)
	if !deadline.IsZero() {
		rem := time.Until(deadline)
		if rem <= 0 {
			return 0, os.ErrDeadlineExceeded
		}
		ms := rem.Milliseconds()
		if ms <= 0 {
			ms = 1
		}
		timeoutMs = uint32(ms)
	}
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(ev)

	var ol windows.Overlapped
	ol.HEvent = ev
	err = op(&ol)
	if err != nil && !errors.Is(err, windows.ERROR_IO_PENDING) {
		return 0, err
	}
	if errors.Is(err, windows.ERROR_IO_PENDING) {
		wait, waitErr := windows.WaitForSingleObject(ev, timeoutMs)
		if waitErr != nil || wait == uint32(windows.WAIT_TIMEOUT) {
			windows.CancelIoEx(c.h, &ol)
			var n uint32
			_ = windows.GetOverlappedResult(c.h, &ol, &n, true)
			if waitErr != nil {
				return int(n), waitErr
			}
			return int(n), os.ErrDeadlineExceeded
		}
	}
	var n uint32
	if err := windows.GetOverlappedResult(c.h, &ol, &n, false); err != nil {
		return int(n), err
	}
	return int(n), nil
}

func (c *winPipeConn) Close() error {
	c.once.Do(func() {
		_ = windows.CancelIoEx(c.h, nil)
		_ = windows.DisconnectNamedPipe(c.h)
		_ = windows.CloseHandle(c.h)
	})
	return nil
}

func (c *winPipeConn) LocalAddr() net.Addr  { return c.addr }
func (c *winPipeConn) RemoteAddr() net.Addr { return c.addr }

func (c *winPipeConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	c.readDeadline = t
	c.writeDeadline = t
	c.mu.Unlock()
	return nil
}

func (c *winPipeConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	c.readDeadline = t
	c.mu.Unlock()
	return nil
}

func (c *winPipeConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	c.writeDeadline = t
	c.mu.Unlock()
	return nil
}
