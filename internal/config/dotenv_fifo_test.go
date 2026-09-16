//go:build linux || darwin

package config

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Opening a named pipe blocks until a writer appears, so configuration loading must skip it.
func TestEnvFilesSkipNamedPipes(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	for _, setting := range []string{"", "1"} {
		t.Setenv("CALLM_LOAD_DOTENV", setting)
		done := make(chan error, 1)
		go func() {
			_, err := loadEnvFiles(path, []string{path})
			done <- err
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("blocked on a named pipe with CALLM_LOAD_DOTENV=%q", setting)
		}
	}
}
