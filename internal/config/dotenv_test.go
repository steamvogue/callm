package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeEnvFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A current-directory .env loads only when the environment or a trusted file opts in.
func TestWorkspaceDotenvLoading(t *testing.T) {
	for _, tc := range []struct {
		name               string
		env                map[string]string
		workspace, trusted string
		want               map[string]string
		notice, fail       bool
	}{
		{name: "ignored by default", workspace: "CALLM_BASE_URL=http://attacker.invalid/v1\nPOOLSIDE_API_KEY=workspace", trusted: "STRAITLY_API_KEY=trusted",
			want: map[string]string{"CALLM_BASE_URL": "", "POOLSIDE_API_KEY": "", "STRAITLY_API_KEY": "trusted"}, notice: true},
		{name: "cannot enable itself", workspace: "CALLM_LOAD_DOTENV=1\nCALLM_BASE_URL=http://attacker.invalid/v1",
			want: map[string]string{"CALLM_LOAD_DOTENV": "", "CALLM_BASE_URL": ""}, notice: true},
		{name: "unrelated variables", workspace: "DB_PASSWORD=secret", want: map[string]string{"DB_PASSWORD": ""}},
		{name: "already set", env: map[string]string{"STRAITLY_API_KEY": "env"}, workspace: "STRAITLY_API_KEY=workspace",
			want: map[string]string{"STRAITLY_API_KEY": "env"}},
		{name: "environment opt-in keeps precedence", env: map[string]string{"CALLM_LOAD_DOTENV": "1"}, workspace: "CALLM_MODEL=workspace\nCALLM_BASE_URL=http://project.invalid/v1", trusted: "CALLM_MODEL=trusted",
			want: map[string]string{"CALLM_MODEL": "workspace", "CALLM_BASE_URL": "http://project.invalid/v1"}},
		{name: "trusted file opt-in", workspace: "CALLM_MODEL=workspace", trusted: "CALLM_LOAD_DOTENV=true\nCALLM_MODEL=trusted",
			want: map[string]string{"CALLM_MODEL": "workspace"}},
		{name: "environment wins", env: map[string]string{"CALLM_LOAD_DOTENV": "1", "CALLM_MODEL": "env"}, workspace: "CALLM_MODEL=workspace",
			want: map[string]string{"CALLM_MODEL": "env"}},
		{name: "opt-out is silent", env: map[string]string{"CALLM_LOAD_DOTENV": "0"}, workspace: "CALLM_BASE_URL=http://attacker.invalid/v1",
			want: map[string]string{"CALLM_BASE_URL": ""}},
		{name: "invalid setting", env: map[string]string{"CALLM_LOAD_DOTENV": "maybe"}, workspace: "CALLM_BASE_URL=http://attacker.invalid/v1",
			want: map[string]string{"CALLM_BASE_URL": ""}, fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// t.Setenv restores every variable the loader may change.
			for _, key := range []string{"CALLM_LOAD_DOTENV", "CALLM_BASE_URL", "CALLM_MODEL", "POOLSIDE_API_KEY", "STRAITLY_API_KEY", "DB_PASSWORD"} {
				t.Setenv(key, tc.env[key])
			}
			trusted := []string{filepath.Join(t.TempDir(), "missing"), writeEnvFile(t, tc.trusted)}
			notice, err := loadEnvFiles(writeEnvFile(t, tc.workspace), trusted)
			if (err != nil) != tc.fail || (notice != "") != tc.notice || (tc.notice && !strings.Contains(notice, "CALLM_LOAD_DOTENV=1")) {
				t.Fatalf("notice=%q err=%v", notice, err)
			}
			for key, want := range tc.want {
				if got := os.Getenv(key); got != want {
					t.Errorf("%s=%q, want %q", key, got, want)
				}
			}
		})
	}
}

// Running a checkout's bin/callm from its root reads that .env as the trusted executable file.
func TestWorkspaceDotenvSameAsTrusted(t *testing.T) {
	t.Setenv("CALLM_LOAD_DOTENV", "")
	t.Setenv("CALLM_MODEL", "")
	path := writeEnvFile(t, "CALLM_MODEL=repository")
	notice, err := loadEnvFiles(path, []string{path})
	if err != nil || notice != "" || os.Getenv("CALLM_MODEL") != "repository" {
		t.Fatalf("notice=%q err=%v model=%q", notice, err, os.Getenv("CALLM_MODEL"))
	}
}
