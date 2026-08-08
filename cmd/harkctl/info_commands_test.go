package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hark/internal/ipc"
)

func TestStatusRejectsIncompatibleDaemonProtocol(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	socketDir := filepath.Join(t.TempDir(), "runtime")
	if err := os.Mkdir(socketDir, 0o700); err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(socketDir, "harkd.sock")
	server := ipc.Server{
		SocketPath: socketPath,
		Handler: func(_ context.Context, request ipc.Request) (any, error) {
			return ipc.Status{Name: "hark", Version: "old", ProtocolVersion: 1}, nil
		},
	}
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Serve(ctx)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, statErr := os.Stat(socketPath); statErr == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	err := status(ctx, socketPath, []string{"--json", "--require-protocol", "2"})
	if err == nil || !strings.Contains(err.Error(), "protocol version 1 is incompatible") {
		t.Fatalf("status error = %v", err)
	}
	var coded interface{ ExitCode() int }
	if !errors.As(err, &coded) || coded.ExitCode() != ipc.IncompatibleProtocolExitCode {
		t.Fatalf("status error exit code = %v, want %d", coded, ipc.IncompatibleProtocolExitCode)
	}

	cancel()
	if serveErr := <-serverDone; serveErr != nil {
		t.Fatalf("serve: %v", serveErr)
	}
}
