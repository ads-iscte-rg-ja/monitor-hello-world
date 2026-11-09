package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMissingBufferIsNotCreated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unprepared.db")
	if err := run(context.Background(), "logs", path, 1); err == nil {
		t.Fatal("expected initialization error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("application must not initialize a buffer")
	}
}

func TestRecoveredPanicRecordsDiagnostic(t *testing.T) {
	called := false
	if err := capturePanic(func(message, stack string) error {
		called = true
		if message == "" || stack == "" {
			t.Fatal("panic diagnostics missing")
		}
		return nil
	}); err != nil || !called {
		t.Fatal("panic was not recorded", err)
	}
}
