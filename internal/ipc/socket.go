package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const (
	defaultMaxRequestBytes    int64 = 1 << 20
	defaultRequestReadTimeout       = 5 * time.Second
	defaultMaxConnections           = 64
)

var ErrServerAlreadyRunning = errors.New("ipc server is already running")

type Handler func(context.Context, Request) (any, error)
type StreamHandler func(context.Context, Request, func(any) error) error

type Server struct {
	SocketPath         string
	Handler            Handler
	StreamHandler      StreamHandler
	MaxRequestBytes    int64
	RequestReadTimeout time.Duration
	MaxConnections     int
}

func DefaultSocketPath() string {
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "hark", "harkd.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("hark-%d", os.Getuid()), "harkd.sock")
}

func (s *Server) Serve(ctx context.Context) error {
	if s.SocketPath == "" {
		s.SocketPath = DefaultSocketPath()
	}
	if s.Handler == nil {
		return errors.New("ipc server requires a handler")
	}
	if !filepath.IsAbs(s.SocketPath) {
		return fmt.Errorf("ipc socket path must be absolute: %q", s.SocketPath)
	}

	if err := ensureSocketDirectory(filepath.Dir(s.SocketPath)); err != nil {
		return err
	}
	if err := prepareSocketPath(s.SocketPath); err != nil {
		return err
	}

	listener, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.SocketPath, err)
	}
	if unixListener, ok := listener.(*net.UnixListener); ok {
		unixListener.SetUnlinkOnClose(false)
	}
	defer listener.Close()
	socketInfo, err := os.Lstat(s.SocketPath)
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("inspect created socket: %w", err)
	}
	defer removeSocketIfOwned(s.SocketPath, socketInfo)
	if err := os.Chmod(s.SocketPath, 0o600); err != nil {
		_ = listener.Close()
		return fmt.Errorf("set socket permissions: %w", err)
	}

	stopListener := context.AfterFunc(ctx, func() {
		_ = listener.Close()
	})
	defer stopListener()

	maxConnections := s.MaxConnections
	if maxConnections <= 0 {
		maxConnections = defaultMaxConnections
	}
	slots := make(chan struct{}, maxConnections)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept ipc connection: %w", err)
		}
		select {
		case slots <- struct{}{}:
			go func() {
				defer func() { <-slots }()
				s.handleConn(ctx, conn)
			}()
		default:
			_ = conn.Close()
		}
	}
}

func (s *Server) handleConn(serverCtx context.Context, conn net.Conn) {
	defer conn.Close()

	ctx, cancel := context.WithCancel(serverCtx)
	defer cancel()
	stopClose := context.AfterFunc(serverCtx, func() {
		_ = conn.Close()
	})
	defer stopClose()

	readTimeout := s.RequestReadTimeout
	if readTimeout <= 0 {
		readTimeout = defaultRequestReadTimeout
	}
	if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		return
	}

	maxRequestBytes := s.MaxRequestBytes
	if maxRequestBytes <= 0 {
		maxRequestBytes = defaultMaxRequestBytes
	}
	limited := &io.LimitedReader{R: conn, N: maxRequestBytes + 1}
	var req Request
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	encoder := json.NewEncoder(conn)

	if err := decoder.Decode(&req); err != nil {
		message := fmt.Sprintf("decode request: %v", err)
		if limited.N <= 0 {
			message = fmt.Sprintf("request exceeds %d bytes", maxRequestBytes)
		}
		_ = encoder.Encode(Response{OK: false, Error: message})
		return
	}
	if limited.N <= 0 {
		_ = encoder.Encode(Response{OK: false, Error: fmt.Sprintf("request exceeds %d bytes", maxRequestBytes)})
		return
	}
	if err := validateMethod(req.Method); err != nil {
		_ = encoder.Encode(Response{OK: false, Error: err.Error()})
		return
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return
	}

	go cancelWhenClientDisconnects(ctx, cancel, conn)

	result, err := s.Handler(ctx, req)
	if s.StreamHandler != nil && err != nil && errors.Is(err, ErrUseStream) {
		err = s.StreamHandler(ctx, req, func(event any) error {
			if err := encoder.Encode(event); err != nil {
				cancel()
				return err
			}
			return nil
		})
		if err != nil {
			_ = encoder.Encode(Response{OK: false, Error: err.Error()})
		}
		return
	}
	if err != nil {
		_ = encoder.Encode(Response{OK: false, Error: err.Error()})
		return
	}

	_ = encoder.Encode(Response{OK: true, Result: result})
}

func ensureSocketDirectory(directory string) error {
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return fmt.Errorf("create socket directory: %w", err)
		}
		info, err = os.Lstat(directory)
	}
	if err != nil {
		return fmt.Errorf("inspect socket directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("socket parent is not a directory: %s", directory)
	}
	if err := requireCurrentUserOwner(info, "socket directory"); err != nil {
		return err
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("socket directory %s must not be accessible by group or others (mode %04o)", directory, info.Mode().Perm())
	}
	return nil
}

func prepareSocketPath(socketPath string) error {
	info, err := os.Lstat(socketPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect socket path: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to replace non-socket path %s", socketPath)
	}
	if err := requireCurrentUserOwner(info, "socket"); err != nil {
		return err
	}

	conn, dialErr := net.DialTimeout("unix", socketPath, 250*time.Millisecond)
	if dialErr == nil {
		_ = conn.Close()
		return fmt.Errorf("%w at %s", ErrServerAlreadyRunning, socketPath)
	}
	if errors.Is(dialErr, os.ErrNotExist) {
		return nil
	}
	if !errors.Is(dialErr, syscall.ECONNREFUSED) {
		return fmt.Errorf("cannot verify whether socket %s is stale; refusing to remove it: %w", socketPath, dialErr)
	}
	if err := os.Remove(socketPath); err != nil {
		return fmt.Errorf("remove stale socket: %w", err)
	}
	return nil
}

func requireCurrentUserOwner(info os.FileInfo, description string) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot determine %s owner", description)
	}
	currentUID := os.Getuid()
	if currentUID < 0 || int64(currentUID) > int64(^uint32(0)) {
		return fmt.Errorf("current uid %d is outside the platform uid range", currentUID)
	}
	if stat.Uid != uint32(currentUID) {
		return fmt.Errorf("%s is owned by uid %d, expected %d", description, stat.Uid, currentUID)
	}
	return nil
}

func removeSocketIfOwned(socketPath string, expected os.FileInfo) error {
	info, err := os.Lstat(socketPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to remove non-socket path %s", socketPath)
	}
	if expected != nil && !os.SameFile(info, expected) {
		return fmt.Errorf("refusing to remove replaced socket %s", socketPath)
	}
	return os.Remove(socketPath)
}

func cancelWhenClientDisconnects(ctx context.Context, cancel context.CancelFunc, conn net.Conn) {
	var buffer [1]byte
	for {
		if _, err := conn.Read(buffer[:]); err != nil {
			cancel()
			return
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func Call(ctx context.Context, socketPath, method string, params any, result any) error {
	if socketPath == "" {
		socketPath = DefaultSocketPath()
	}
	if err := validateMethod(method); err != nil {
		return err
	}

	var rawParams json.RawMessage
	if params != nil {
		encoded, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("encode params: %w", err)
		}
		rawParams = encoded
	}

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return fmt.Errorf("connect to harkd at %s: %w", socketPath, err)
	}
	defer conn.Close()
	stopCancel := closeConnectionOnCancel(ctx, conn)
	defer stopCancel()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(Request{Method: method, Params: rawParams}); err != nil {
		return connectionError(ctx, "send request", err)
	}

	var res Response
	if err := decoder.Decode(&res); err != nil {
		return connectionError(ctx, "decode response", err)
	}
	if !res.OK {
		if res.Error == "" {
			return errors.New("request failed")
		}
		return errors.New(res.Error)
	}
	if result == nil {
		return nil
	}

	encoded, err := json.Marshal(res.Result)
	if err != nil {
		return fmt.Errorf("encode response result: %w", err)
	}
	if err := json.Unmarshal(encoded, result); err != nil {
		return fmt.Errorf("decode response result: %w", err)
	}

	return nil
}

func validateMethod(method string) error {
	if method == "" || len(method) > 64 {
		return errors.New("ipc method must contain 1 to 64 bytes")
	}
	for _, character := range method {
		switch {
		case character >= 'a' && character <= 'z':
		case character >= '0' && character <= '9':
		case character == '_':
		default:
			return fmt.Errorf("ipc method %q contains an unsupported character", method)
		}
	}
	return nil
}

var ErrUseStream = errors.New("use streaming handler")

func CallStream(ctx context.Context, socketPath, method string, params any, handle func(json.RawMessage) error) error {
	if socketPath == "" {
		socketPath = DefaultSocketPath()
	}
	if err := validateMethod(method); err != nil {
		return err
	}
	if handle == nil {
		return errors.New("stream handler must not be nil")
	}

	var rawParams json.RawMessage
	if params != nil {
		encoded, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("encode params: %w", err)
		}
		rawParams = encoded
	}

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return fmt.Errorf("connect to harkd at %s: %w", socketPath, err)
	}
	defer conn.Close()
	stopCancel := closeConnectionOnCancel(ctx, conn)
	defer stopCancel()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(Request{Method: method, Params: rawParams}); err != nil {
		return connectionError(ctx, "send request", err)
	}

	for {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return connectionError(ctx, "decode stream event", err)
		}
		if err := handle(raw); err != nil {
			return err
		}
	}
}

func closeConnectionOnCancel(ctx context.Context, conn net.Conn) func() bool {
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	return context.AfterFunc(ctx, func() {
		_ = conn.Close()
	})
}

func connectionError(ctx context.Context, operation string, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("%s: %w", operation, ctxErr)
	}
	if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		return fmt.Errorf("%s: %w", operation, context.DeadlineExceeded)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
