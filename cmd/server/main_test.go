package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReturnsClearErrorOnDatabaseInitializationFailure(t *testing.T) {
	t.Setenv("PORT", "0")
	t.Setenv("DB_PATH", filepath.Dir(t.TempDir()))

	err := run()
	if err == nil {
		t.Fatalf("expected run to fail when DB_PATH points to a directory")
	}

	if !strings.Contains(err.Error(), "failed to initialize database at") {
		t.Fatalf("expected clear startup DB error prefix, got: %v", err)
	}
}
