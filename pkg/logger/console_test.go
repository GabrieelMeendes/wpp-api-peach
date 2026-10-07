package logger

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"strings"
	"testing"

	gomessguii "github.com/gomessguii/logger"
)

func captureConsole(t *testing.T, jsonFormat bool) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errOut bytes.Buffer
	stdout, stderr = &out, &errOut
	SetupConsole(jsonFormat)
	t.Cleanup(func() {
		stdout, stderr = os.Stdout, os.Stderr
		SetupConsole(false)
		log.SetOutput(os.Stderr)
	})
	return &out, &errOut
}

func TestStdLogRoutesByLevel(t *testing.T) {
	out, errOut := captureConsole(t, false)

	gomessguii.LogInfo("info msg")
	gomessguii.LogWarn("warn msg")
	gomessguii.LogError("error msg")

	if !strings.Contains(out.String(), "info msg") || !strings.Contains(out.String(), "warn msg") {
		t.Fatalf("info/warn should go to stdout, got stdout=%q", out.String())
	}
	if strings.Contains(out.String(), "error msg") {
		t.Fatalf("error should not go to stdout, got %q", out.String())
	}
	if !strings.Contains(errOut.String(), "error msg") || strings.Contains(errOut.String(), "info msg") {
		t.Fatalf("only error should go to stderr, got %q", errOut.String())
	}
}

func TestStdLogJSON(t *testing.T) {
	out, errOut := captureConsole(t, true)

	gomessguii.LogInfo("hello %d", 1)
	gomessguii.LogError("boom")

	var info, failure consoleEntry
	if err := json.Unmarshal(out.Bytes(), &info); err != nil {
		t.Fatalf("stdout is not JSON: %v (%q)", err, out.String())
	}
	if info.Level != "info" || info.Message != "hello 1" {
		t.Fatalf("unexpected info entry: %+v", info)
	}
	if err := json.Unmarshal(errOut.Bytes(), &failure); err != nil {
		t.Fatalf("stderr is not JSON: %v (%q)", err, errOut.String())
	}
	if failure.Level != "error" || failure.Message != "boom" {
		t.Fatalf("unexpected error entry: %+v", failure)
	}
}

func TestWALoggerRoutesByLevel(t *testing.T) {
	out, errOut := captureConsole(t, false)

	l := NewWALogger("Client", "INFO").Sub("Socket")
	l.Debugf("hidden")
	l.Infof("connected")
	l.Errorf("failed")

	if strings.Contains(out.String(), "hidden") {
		t.Fatalf("debug should be filtered by min level, got %q", out.String())
	}
	if !strings.Contains(out.String(), "[Client/Socket INFO] connected") {
		t.Fatalf("info should go to stdout, got %q", out.String())
	}
	if !strings.Contains(errOut.String(), "[Client/Socket ERROR] failed") {
		t.Fatalf("error should go to stderr, got %q", errOut.String())
	}
}
