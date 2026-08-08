package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCallStatus(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	socketPath := testSocketPath(t)
	server := Server{
		SocketPath: socketPath,
		Handler: func(_ context.Context, req Request) (any, error) {
			if req.Method != "status" {
				t.Fatalf("unexpected method: %s", req.Method)
			}
			return Status{Name: "hark", Version: "test"}, nil
		},
	}

	errs := make(chan error, 1)
	go func() {
		errs <- server.Serve(ctx)
	}()

	var status Status
	err := waitForCall(ctx, socketPath, "status", nil, &status)
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}
	if status.Name != "hark" {
		t.Fatalf("unexpected status name: %q", status.Name)
	}

	cancel()
	if err := <-errs; err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
}

func TestCallStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	socketPath := testSocketPath(t)
	server := Server{
		SocketPath: socketPath,
		Handler: func(_ context.Context, req Request) (any, error) {
			if req.Method == "ask" {
				return nil, ErrUseStream
			}
			return nil, nil
		},
		StreamHandler: func(_ context.Context, req Request, send func(any) error) error {
			if req.Method != "ask" {
				t.Fatalf("unexpected method: %s", req.Method)
			}
			if err := send(map[string]string{"type": "delta", "text": "hello"}); err != nil {
				return err
			}
			return send(map[string]string{"type": "done"})
		},
	}

	errs := make(chan error, 1)
	go func() {
		errs <- server.Serve(ctx)
	}()

	var seen []string
	var err error
	for i := 0; i < 50; i++ {
		seen = nil
		err = CallStream(ctx, socketPath, "ask", nil, func(raw json.RawMessage) error {
			var event struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if err := json.Unmarshal(raw, &event); err != nil {
				return err
			}
			seen = append(seen, event.Type+":"+event.Text)
			return nil
		})
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("CallStream returned error: %v", err)
	}
	if len(seen) != 2 || seen[0] != "delta:hello" || seen[1] != "done:" {
		t.Fatalf("unexpected events: %#v", seen)
	}

	cancel()
	if err := <-errs; err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
}

func TestCallHonorsContextAfterConnecting(t *testing.T) {
	socketPath := testSocketPath(t)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	accepted := make(chan struct{})
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		close(accepted)
		var request Request
		_ = json.NewDecoder(conn).Decode(&request)
		<-time.After(time.Second)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = Call(ctx, socketPath, "status", nil, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Call error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("Call returned too slowly after cancellation: %s", elapsed)
	}
	select {
	case <-accepted:
	default:
		t.Fatal("server did not accept the connection")
	}
}

func TestCallRejectsInvalidMethodBeforeConnecting(t *testing.T) {
	err := Call(context.Background(), testSocketPath(t), "History.List", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported character") {
		t.Fatalf("Call error = %v, want invalid method rejection", err)
	}
}

func TestServerRejectsUnknownRequestFields(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	socketPath := testSocketPath(t)
	var handlerCalls atomic.Int32
	server := Server{
		SocketPath: socketPath,
		Handler: func(context.Context, Request) (any, error) {
			handlerCalls.Add(1)
			return Status{Name: "ready"}, nil
		},
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Serve(ctx)
	}()
	waitForServer(t, ctx, socketPath)
	callsBeforeInvalidRequest := handlerCalls.Load()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	if _, err := conn.Write([]byte("{\"method\":\"status\",\"unknown\":true}\n")); err != nil {
		t.Fatalf("write request: %v", err)
	}
	var response Response
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		t.Fatalf("decode rejection: %v", err)
	}
	_ = conn.Close()
	if response.OK || !strings.Contains(response.Error, `unknown field "unknown"`) {
		t.Fatalf("response = %#v, want unknown field rejection", response)
	}
	if got := handlerCalls.Load(); got != callsBeforeInvalidRequest {
		t.Fatalf("handler calls = %d, want %d; invalid request reached handler", got, callsBeforeInvalidRequest)
	}

	cancel()
	if err := <-serverErrors; err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
}

func TestCallStreamCancellationCancelsServerHandler(t *testing.T) {
	serverCtx, stopServer := context.WithCancel(context.Background())
	socketPath := testSocketPath(t)
	handlerStarted := make(chan struct{})
	handlerCanceled := make(chan struct{})
	server := Server{
		SocketPath: socketPath,
		Handler: func(_ context.Context, req Request) (any, error) {
			if req.Method == "ask" {
				return nil, ErrUseStream
			}
			return nil, nil
		},
		StreamHandler: func(ctx context.Context, _ Request, _ func(any) error) error {
			close(handlerStarted)
			<-ctx.Done()
			close(handlerCanceled)
			return ctx.Err()
		},
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Serve(serverCtx)
	}()
	waitForServer(t, serverCtx, socketPath)

	callCtx, cancelCall := context.WithCancel(context.Background())
	callErrors := make(chan error, 1)
	go func() {
		callErrors <- CallStream(callCtx, socketPath, "ask", nil, func(json.RawMessage) error {
			return nil
		})
	}()

	select {
	case <-handlerStarted:
	case <-time.After(time.Second):
		t.Fatal("stream handler did not start")
	}
	cancelCall()

	select {
	case err := <-callErrors:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("CallStream error = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("CallStream did not return after cancellation")
	}
	select {
	case <-handlerCanceled:
	case <-time.After(time.Second):
		t.Fatal("server handler context was not canceled")
	}

	stopServer()
	if err := <-serverErrors; err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
}

func TestServerRefusesToReplaceDirectoryOrFile(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(*testing.T, string)
	}{
		{
			name: "directory",
			setup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatalf("create target directory: %v", err)
				}
				if err := os.WriteFile(filepath.Join(path, "keep"), []byte("safe"), 0o600); err != nil {
					t.Fatalf("create marker: %v", err)
				}
			},
		},
		{
			name: "regular file",
			setup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("safe"), 0o600); err != nil {
					t.Fatalf("create target file: %v", err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			socketPath := testSocketPath(t)
			test.setup(t, socketPath)
			server := Server{
				SocketPath: socketPath,
				Handler: func(context.Context, Request) (any, error) {
					return nil, nil
				},
			}
			err := server.Serve(context.Background())
			if err == nil || !strings.Contains(err.Error(), "refusing to replace non-socket") {
				t.Fatalf("Serve error = %v, want refusal", err)
			}
			if _, statErr := os.Lstat(socketPath); statErr != nil {
				t.Fatalf("target was removed: %v", statErr)
			}
			if test.name == "directory" {
				if _, statErr := os.Stat(filepath.Join(socketPath, "keep")); statErr != nil {
					t.Fatalf("directory contents were removed: %v", statErr)
				}
			}
		})
	}
}

func TestSecondServerDoesNotUnlinkActiveSocket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	socketPath := testSocketPath(t)
	handler := func(context.Context, Request) (any, error) {
		return Status{Name: "first"}, nil
	}
	first := Server{SocketPath: socketPath, Handler: handler}
	firstErrors := make(chan error, 1)
	go func() {
		firstErrors <- first.Serve(ctx)
	}()
	waitForServer(t, ctx, socketPath)

	second := Server{SocketPath: socketPath, Handler: handler}
	err := second.Serve(context.Background())
	if !errors.Is(err, ErrServerAlreadyRunning) {
		t.Fatalf("second Serve error = %v, want ErrServerAlreadyRunning", err)
	}

	var status Status
	if err := Call(ctx, socketPath, "status", nil, &status); err != nil {
		t.Fatalf("first server became unavailable: %v", err)
	}
	if status.Name != "first" {
		t.Fatalf("unexpected first server response: %#v", status)
	}

	cancel()
	if err := <-firstErrors; err != nil {
		t.Fatalf("first Serve returned error: %v", err)
	}
}

func TestServerReplacesStaleOwnedSocket(t *testing.T) {
	socketPath := testSocketPath(t)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("create stale socket: %v", err)
	}
	unixListener, ok := listener.(*net.UnixListener)
	if !ok {
		t.Fatalf("listener type = %T, want *net.UnixListener", listener)
	}
	unixListener.SetUnlinkOnClose(false)
	if err := listener.Close(); err != nil {
		t.Fatalf("close stale listener: %v", err)
	}
	if _, err := os.Lstat(socketPath); err != nil {
		t.Fatalf("stale socket was not preserved for test: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	server := Server{
		SocketPath: socketPath,
		Handler: func(context.Context, Request) (any, error) {
			return Status{Name: "replacement"}, nil
		},
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Serve(ctx)
	}()
	waitForServer(t, ctx, socketPath)

	var status Status
	if err := Call(ctx, socketPath, "status", nil, &status); err != nil {
		t.Fatalf("call replacement server: %v", err)
	}
	if status.Name != "replacement" {
		t.Fatalf("unexpected replacement response: %#v", status)
	}

	cancel()
	if err := <-serverErrors; err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
}

func TestServerUsesPrivateSocketPermissions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	socketPath := testSocketPath(t)
	server := Server{
		SocketPath: socketPath,
		Handler: func(context.Context, Request) (any, error) {
			return map[string]bool{"ok": true}, nil
		},
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Serve(ctx)
	}()
	waitForServer(t, ctx, socketPath)

	info, err := os.Lstat(socketPath)
	if err != nil {
		t.Fatalf("inspect socket: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("socket mode = %04o, want 0600", got)
	}

	cancel()
	if err := <-serverErrors; err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
}

func TestServerRejectsOversizedRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	socketPath := testSocketPath(t)
	server := Server{
		SocketPath:      socketPath,
		MaxRequestBytes: 128,
		Handler: func(context.Context, Request) (any, error) {
			return Status{Name: "ready"}, nil
		},
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Serve(ctx)
	}()
	waitForServer(t, ctx, socketPath)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	_, err = conn.Write([]byte(`{"method":"ask","params":{"prompt":"` + strings.Repeat("x", 1024) + `"}}` + "\n"))
	if err != nil {
		t.Fatalf("write oversized request: %v", err)
	}
	var response Response
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		t.Fatalf("decode rejection: %v", err)
	}
	_ = conn.Close()
	if response.OK || !strings.Contains(response.Error, "exceeds 128 bytes") {
		t.Fatalf("unexpected rejection: %#v", response)
	}

	cancel()
	if err := <-serverErrors; err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
}

func TestServerTimesOutIncompleteRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	socketPath := testSocketPath(t)
	server := Server{
		SocketPath:         socketPath,
		RequestReadTimeout: 40 * time.Millisecond,
		Handler: func(context.Context, Request) (any, error) {
			return Status{Name: "ready"}, nil
		},
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Serve(ctx)
	}()
	waitForServer(t, ctx, socketPath)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(`{"method":`)); err != nil {
		t.Fatalf("write partial request: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	var response Response
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		t.Fatalf("decode timeout response: %v", err)
	}
	if response.OK || !strings.Contains(response.Error, "i/o timeout") {
		t.Fatalf("unexpected timeout response: %#v", response)
	}

	cancel()
	if err := <-serverErrors; err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
}

func testSocketPath(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatalf("secure temp directory: %v", err)
	}
	return filepath.Join(directory, "harkd.sock")
}

func waitForServer(t *testing.T, ctx context.Context, socketPath string) {
	t.Helper()
	var status Status
	if err := waitForCall(ctx, socketPath, "status", nil, &status); err != nil {
		t.Fatalf("wait for server: %v", err)
	}
}

func waitForCall(ctx context.Context, socketPath, method string, params any, result any) error {
	var err error
	for i := 0; i < 100; i++ {
		err = Call(ctx, socketPath, method, params, result)
		if err == nil {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return err
}
