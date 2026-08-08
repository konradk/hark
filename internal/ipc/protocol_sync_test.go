package ipc

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"testing"
)

func TestPluginProtocolVersionMatchesDaemon(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	servicePath := filepath.Join(filepath.Dir(sourceFile), "..", "..", "plugin", "Service.qml")
	service, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("read plugin service: %v", err)
	}

	match := regexp.MustCompile(`readonly property int protocolVersion:\s*([0-9]+)`).FindSubmatch(service)
	if len(match) != 2 {
		t.Fatal("plugin service does not declare protocolVersion")
	}
	pluginVersion, err := strconv.Atoi(string(match[1]))
	if err != nil {
		t.Fatalf("parse plugin protocol version: %v", err)
	}
	if pluginVersion != ProtocolVersion {
		t.Fatalf("plugin protocol version = %d, daemon protocol version = %d", pluginVersion, ProtocolVersion)
	}

	exitMatch := regexp.MustCompile(`readonly property int incompatibleProtocolExitCode:\s*([0-9]+)`).FindSubmatch(service)
	if len(exitMatch) != 2 {
		t.Fatal("plugin service does not declare incompatibleProtocolExitCode")
	}
	pluginExitCode, err := strconv.Atoi(string(exitMatch[1]))
	if err != nil {
		t.Fatalf("parse plugin incompatible protocol exit code: %v", err)
	}
	if pluginExitCode != IncompatibleProtocolExitCode {
		t.Fatalf("plugin incompatible protocol exit code = %d, CLI exit code = %d", pluginExitCode, IncompatibleProtocolExitCode)
	}
}
