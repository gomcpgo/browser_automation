package browser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChromePathFromEnv(t *testing.T) {
	t.Run("unset means auto-detect", func(t *testing.T) {
		t.Setenv(ChromePathEnv, "")

		path, err := ChromePathFromEnv()
		if err != nil || path != "" {
			t.Fatalf("got %q, %v", path, err)
		}
	})

	t.Run("existing binary is returned", func(t *testing.T) {
		bin := filepath.Join(t.TempDir(), "chrome")
		if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatal(err)
		}
		t.Setenv(ChromePathEnv, bin)

		path, err := ChromePathFromEnv()
		if err != nil || path != bin {
			t.Fatalf("got %q, %v", path, err)
		}
	})

	t.Run("missing path is an error", func(t *testing.T) {
		t.Setenv(ChromePathEnv, filepath.Join(t.TempDir(), "nope"))

		if _, err := ChromePathFromEnv(); err == nil {
			t.Fatal("expected an error for a path that does not exist")
		}
	})

	t.Run("app bundle directory is an error with a hint", func(t *testing.T) {
		t.Setenv(ChromePathEnv, t.TempDir())

		_, err := ChromePathFromEnv()
		if err == nil {
			t.Fatal("expected an error for a directory")
		}
		if !strings.Contains(err.Error(), "Contents/MacOS") {
			t.Fatalf("expected the bundle hint, got %v", err)
		}
	})
}
