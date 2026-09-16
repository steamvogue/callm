package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A checkout's .env must not redirect keys from the environment or user configuration.
// Both endpoints are local test servers; nothing leaves the host.
func TestWorkspaceDotenvCannotRedirectKeys(t *testing.T) {
	st := []string{"--st", "--no-stdin", "hello"}
	for _, tc := range []struct {
		name                       string
		args, env                  []string
		config, dotenv             string
		trustedAuth, workspaceAuth string // expected Authorization per endpoint; empty means no request
		output                     string
		notice, fail               bool
	}{
		// The reported case: no flags, a Poolside key, and only a .env in the directory.
		// A dead proxy keeps the real Poolside endpoint unreachable.
		{name: "reported reproduction", args: []string{"--no-stdin", "hello"}, env: []string{"POOLSIDE_API_KEY=sk-canary", "HTTPS_PROXY=http://127.0.0.1:1"},
			dotenv: "CALLM_BASE_URL={workspace}/v1", notice: true, fail: true},
		{name: "key from environment", args: st, env: []string{"STRAITLY_API_KEY=env-canary", "STRAITLY_BASE_URL={trusted}"},
			dotenv: "CALLM_BASE_URL={workspace}", trustedAuth: "Bearer env-canary", output: "answer", notice: true},
		{name: "key from user config", args: st, config: "STRAITLY_API_KEY=config-canary\nSTRAITLY_BASE_URL={trusted}",
			dotenv: "CALLM_BASE_URL={workspace}", trustedAuth: "Bearer config-canary", output: "answer", notice: true},
		{name: "dotenv cannot enable itself", args: st, env: []string{"STRAITLY_API_KEY=env-canary", "STRAITLY_BASE_URL={trusted}"},
			dotenv: "CALLM_LOAD_DOTENV=1\nCALLM_BASE_URL={workspace}", trustedAuth: "Bearer env-canary", output: "answer", notice: true},
		{name: "opt-out hides notice", args: st, env: []string{"CALLM_LOAD_DOTENV=0", "STRAITLY_API_KEY=env-canary", "STRAITLY_BASE_URL={trusted}"},
			dotenv: "CALLM_BASE_URL={workspace}", trustedAuth: "Bearer env-canary", output: "answer"},
		{name: "environment opt-in", args: st, env: []string{"CALLM_LOAD_DOTENV=1"},
			dotenv: "STRAITLY_API_KEY=project-key\nCALLM_BASE_URL={workspace}", workspaceAuth: "Bearer project-key", output: "answer"},
		{name: "user config opt-in", args: st, config: "CALLM_LOAD_DOTENV=true",
			dotenv: "STRAITLY_API_KEY=project-key\nCALLM_BASE_URL={workspace}", workspaceAuth: "Bearer project-key", output: "answer"},
		{name: "invalid setting", args: st, env: []string{"CALLM_LOAD_DOTENV=maybe", "STRAITLY_API_KEY=env-canary", "STRAITLY_BASE_URL={trusted}"},
			dotenv: "CALLM_BASE_URL={workspace}", output: "CALLM_LOAD_DOTENV must be", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			serve := func(seen chan<- string) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					seen <- r.Header.Get("Authorization")
					io.WriteString(w, `{"choices":[{"message":{"content":"answer"}}]}`)
				}))
			}
			trustedSeen, workspaceSeen := make(chan string, 4), make(chan string, 4)
			trusted, workspace := serve(trustedSeen), serve(workspaceSeen)
			defer trusted.Close()
			defer workspace.Close()
			expand := strings.NewReplacer("{trusted}", trusted.URL, "{workspace}", workspace.URL).Replace

			dir, home := t.TempDir(), t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(expand(tc.dotenv)+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.config != "" {
				configDir := filepath.Join(home, ".config", "callm")
				if err := os.MkdirAll(configDir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(configDir, "config"), []byte(expand(tc.config)+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			cmd := testCLI(t, tc.args...)
			cmd.Dir = dir
			cmd.Env = []string{"CALLM_TIMEOUT_TEST_HELPER=1", "HOME=" + home}
			for _, value := range tc.env {
				cmd.Env = append(cmd.Env, expand(value))
			}
			out, err := cmd.CombinedOutput()
			if (err != nil) != tc.fail || !strings.Contains(string(out), tc.output) || strings.Contains(string(out), "CALLM_LOAD_DOTENV=1 loads it") != tc.notice {
				t.Fatalf("err=%v output=%s", err, out)
			}
			for _, endpoint := range []struct {
				name string
				seen chan string
				want string
			}{{"trusted", trustedSeen, tc.trustedAuth}, {"workspace", workspaceSeen, tc.workspaceAuth}} {
				got := make([]string, len(endpoint.seen))
				for i := range got {
					got[i] = <-endpoint.seen
				}
				if (endpoint.want == "" && len(got) != 0) || (endpoint.want != "" && (len(got) != 1 || got[0] != endpoint.want)) {
					t.Errorf("%s endpoint received %q, want %q", endpoint.name, got, endpoint.want)
				}
			}
		})
	}
}
