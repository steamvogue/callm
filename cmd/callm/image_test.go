package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type imageMock struct {
	*httptest.Server
	mu     sync.Mutex
	bodies []map[string]interface{}
	paths  []string
}

func (m *imageMock) last(t *testing.T) map[string]interface{} {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.bodies) == 0 {
		t.Fatal("no image request reached the mock")
	}
	return m.bodies[len(m.bodies)-1]
}

func (m *imageMock) posts() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.bodies)
}

func newImageMock(t *testing.T, response string) *imageMock {
	t.Helper()
	mock := &imageMock{}
	mock.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mock.mu.Lock()
		mock.paths = append(mock.paths, r.Method+" "+r.URL.Path)
		mock.mu.Unlock()
		if r.Method == http.MethodGet {
			io.WriteString(w, `{"data":[{"id":"acme/paint","architecture":{"input_modalities":["text","image"]},"supported_parameters":{"seed":{"type":"boolean"},"input_references":{"type":"boolean"},"aspect_ratio":{"type":"enum","values":["1:1"]}},"supports_streaming":false},{"id":"acme/sketch:free","supported_parameters":{"n":{"type":"range","min":1,"max":4}}}]}`)
			return
		}
		var body map[string]interface{}
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &body)
		mock.mu.Lock()
		mock.bodies = append(mock.bodies, body)
		mock.mu.Unlock()
		io.WriteString(w, response)
	}))
	t.Cleanup(mock.Close)
	return mock
}

func imageCLI(t *testing.T, mock *imageMock, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := testCLI(t, append([]string{"image", "--api", mock.URL}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(cmd.Env, "CALLM_API_KEY=", "CALLM_BASE_URL=", "CALLM_PROXY=", "OPENROUTER_REGION=", "CALLM_IMAGE_MODEL=", "OPENROUTER_API_KEY=openrouter-dummy")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

var (
	pngBytes = []byte("\x89PNG\r\n\x1a\nfake-image")
	pngB64   = base64.StdEncoding.EncodeToString(pngBytes)
)

func TestImageCLIWritesFilesAndSendsReferences(t *testing.T) {
	mock := newImageMock(t, `{"created":7,"data":[{"b64_json":"`+pngB64+`","media_type":"image/png"}],"usage":{"total_tokens":5,"cost":0.04}}`)
	dir := t.TempDir()
	reference := filepath.Join(dir, "ref.jpg")
	if err := os.WriteFile(reference, []byte("jpeg-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := imageCLI(t, mock, dir, "-m", "acme/paint", "-o", "out", "--ref", reference, "--image", "https://example.com/a.png",
		"--aspect-ratio", "16:9", "--resolution", "2K", "--quality", "high", "--format", "png", "--background", "transparent",
		"-n", "1", "--seed", "0", "--param", "output_compression=80", "--param", `provider={"only":["acme"]}`, "--param", "user=abc",
		"--stats", "a", "red", "panda")
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if !strings.Contains(out, "out.png\n") || !strings.Contains(out, "[stats] model=acme/paint images=1") || !strings.Contains(out, "tokens=5 cost=$0.04") {
		t.Fatalf("output: %s", out)
	}
	saved, err := os.ReadFile(filepath.Join(dir, "out.png"))
	if err != nil || string(saved) != string(pngBytes) {
		t.Fatalf("saved file: %v %q", err, saved)
	}
	body := mock.last(t)
	want := map[string]interface{}{"model": "acme/paint", "prompt": "a red panda", "aspect_ratio": "16:9", "resolution": "2K", "quality": "high",
		"output_format": "png", "background": "transparent", "n": 1.0, "seed": 0.0, "output_compression": 80.0, "user": "abc"}
	for key, value := range want {
		if body[key] != value {
			t.Fatalf("%s = %v, want %v", key, body[key], value)
		}
	}
	if _, present := body["stream"]; present {
		t.Fatal("stream must not be sent")
	}
	if only := body["provider"].(map[string]interface{})["only"].([]interface{}); only[0] != "acme" {
		t.Fatalf("provider: %v", body["provider"])
	}
	references := body["input_references"].([]interface{})
	first := references[0].(map[string]interface{})
	local := first["image_url"].(map[string]interface{})["url"].(string)
	remote := references[1].(map[string]interface{})["image_url"].(map[string]interface{})["url"]
	if first["type"] != "image_url" || local != "data:image/jpeg;base64,"+base64.StdEncoding.EncodeToString([]byte("jpeg-bytes")) || remote != "https://example.com/a.png" {
		t.Fatalf("references: %v", references)
	}
	if mock.paths[0] != "POST /images" {
		t.Fatalf("path: %v", mock.paths)
	}
}

func TestImageCLIMultipleJSONAndFormats(t *testing.T) {
	svg := base64.StdEncoding.EncodeToString([]byte("<svg/>"))
	mock := newImageMock(t, `{"data":[{"b64_json":"`+pngB64+`"},{"b64_json":"`+svg+`","media_type":"image/svg+xml"},{"url":"https://cdn.example/x.png","media_type":"image/png"}]}`)
	dir := t.TempDir()
	cmd := testCLI(t, "image", "--api", mock.URL, "-m", "acme/paint", "-o", "set", "--json")
	cmd.Dir = dir
	cmd.Env = append(cmd.Env, "CALLM_API_KEY=", "CALLM_BASE_URL=", "CALLM_PROXY=", "OPENROUTER_REGION=", "OPENROUTER_API_KEY=openrouter-dummy")
	cmd.Stdin = strings.NewReader("prompt from stdin\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	var summary struct {
		Model string
		Files []imageFile
	}
	if err := json.Unmarshal(out, &summary); err != nil {
		t.Fatalf("summary: %v: %s", err, out)
	}
	if summary.Model != "acme/paint" || len(summary.Files) != 3 || summary.Files[0].Path != "set-1.png" || summary.Files[0].MediaType != "image/png" ||
		summary.Files[1].Path != "set-2.svg" || summary.Files[2].URL != "https://cdn.example/x.png" || summary.Files[0].Bytes != len(pngBytes) {
		t.Fatalf("summary: %+v", summary)
	}
	if mock.last(t)["prompt"] != "prompt from stdin" {
		t.Fatalf("prompt: %v", mock.last(t)["prompt"])
	}
	if strings.Contains(string(out), pngB64) {
		t.Fatal("summary must not carry image data")
	}
}

func TestImageCLIProtectsExistingFiles(t *testing.T) {
	mock := newImageMock(t, `{"data":[{"b64_json":"`+pngB64+`","media_type":"image/png"}]}`)
	dir := t.TempDir()
	for _, name := range []string{"keep.png", "late.png"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A known name is refused before any billed request.
	out, err := imageCLI(t, mock, dir, "-m", "acme/paint", "-o", "keep.png", "x")
	if err == nil || !strings.Contains(out, "exists; use --force") || mock.posts() != 0 {
		t.Fatalf("err=%v posts=%d output=%s", err, mock.posts(), out)
	}
	out, err = imageCLI(t, mock, dir, "-m", "acme/paint", "-o", "missing/x.png", "x")
	if err == nil || !strings.Contains(out, "does not exist") || mock.posts() != 0 {
		t.Fatalf("err=%v posts=%d output=%s", err, mock.posts(), out)
	}
	// The extension is known only after the response: keep both files.
	out, err = imageCLI(t, mock, dir, "-m", "acme/paint", "-o", "late", "x")
	if err != nil || !strings.Contains(out, "late.1.png") {
		t.Fatalf("err=%v output=%s", err, out)
	}
	if kept, _ := os.ReadFile(filepath.Join(dir, "late.png")); string(kept) != "original" {
		t.Fatalf("late.png was replaced: %q", kept)
	}
	out, err = imageCLI(t, mock, dir, "-m", "acme/paint", "-o", "keep.png", "--force", "x")
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if replaced, _ := os.ReadFile(filepath.Join(dir, "keep.png")); string(replaced) != string(pngBytes) {
		t.Fatalf("keep.png: %q", replaced)
	}
	out, err = imageCLI(t, mock, dir, "-m", "acme/paint", "-o", "named.jpg", "x")
	if err != nil || !strings.Contains(out, "the image is image/png but the file name ends in .jpg") {
		t.Fatalf("err=%v output=%s", err, out)
	}
}

func TestImageCLIValidation(t *testing.T) {
	mock := newImageMock(t, `{"data":[]}`)
	dir := t.TempDir()
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"model", []string{"x"}, "image requires -m MODEL"},
		{"prompt", []string{"-m", "acme/paint"}, "image requires a prompt"},
		{"count", []string{"-m", "acme/paint", "-n", "11", "x"}, "-n must be between 1 and 10"},
		{"reserved", []string{"-m", "acme/paint", "--param", "stream=true", "x"}, `--param cannot set "stream"`},
		{"param shape", []string{"-m", "acme/paint", "--param", "novalue", "x"}, "must be KEY=VALUE"},
		{"reference", []string{"-m", "acme/paint", "--ref", "absent.png", "x"}, "failed to encode image"},
		{"other preset", []string{"--orca", "-m", "acme/paint", "x"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := testCLI(t, append([]string{"image", "--api", mock.URL}, tc.args...)...)
			if tc.name == "other preset" {
				cmd = testCLI(t, append([]string{"image"}, tc.args...)...)
				tc.want = "image uses OpenRouter's Image API"
			}
			cmd.Dir = dir
			cmd.Stdin = strings.NewReader("")
			cmd.Env = append(cmd.Env, "CALLM_API_KEY=", "CALLM_BASE_URL=", "CALLM_PROXY=", "OPENROUTER_REGION=", "CALLM_IMAGE_MODEL=", "ORCA_API_KEY=orca-dummy", "OPENROUTER_API_KEY=openrouter-dummy")
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), tc.want) || mock.posts() != 0 {
				t.Fatalf("err=%v posts=%d output=%s", err, mock.posts(), out)
			}
		})
	}
	out, err := imageCLI(t, mock, dir, "-m", "acme/paint", "x")
	if err == nil || !strings.Contains(out, "contains no images") {
		t.Fatalf("err=%v output=%s", err, out)
	}
	failing := newImageMock(t, `{"error":{"message":"model does not support input_references","code":400}}`)
	out, err = imageCLI(t, failing, dir, "-m", "acme/paint", "x")
	if err == nil || !strings.Contains(out, "model does not support input_references") {
		t.Fatalf("err=%v output=%s", err, out)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("failed runs left files: %v", entries)
	}
}

func TestImageCLIListAndEnvironmentModel(t *testing.T) {
	mock := newImageMock(t, `{"data":[{"b64_json":"`+pngB64+`","media_type":"image/png"}]}`)
	dir := t.TempDir()
	out, err := imageCLI(t, mock, dir, "--list")
	if err != nil || !strings.Contains(out, "acme/paint") || !strings.Contains(out, "yes") || !strings.Contains(out, "aspect_ratio,seed") || !strings.Contains(out, "acme/sketch:free") {
		t.Fatalf("err=%v output=%s", err, out)
	}
	out, err = imageCLI(t, mock, dir, "--list", "--json", "FREE")
	if err != nil || strings.Contains(out, "acme/paint") || !strings.Contains(out, `"id": "acme/sketch:free"`) {
		t.Fatalf("err=%v output=%s", err, out)
	}
	if mock.posts() != 0 || mock.paths[0] != "GET /images/models" {
		t.Fatalf("list must only read the catalog: %v", mock.paths)
	}
	cmd := testCLI(t, "image", "--api", mock.URL, "-o", "env", "x")
	cmd.Dir = dir
	cmd.Env = append(cmd.Env, "CALLM_API_KEY=", "CALLM_BASE_URL=", "CALLM_PROXY=", "OPENROUTER_REGION=", "OPENROUTER_API_KEY=openrouter-dummy", "CALLM_IMAGE_MODEL=acme/sketch:free")
	if output, err := cmd.CombinedOutput(); err != nil || mock.last(t)["model"] != "acme/sketch:free" {
		t.Fatalf("err=%v output=%s", err, output)
	}
}

func TestImageDispatch(t *testing.T) {
	name, rest := splitCommand([]string{"--or", "image", "-m", "x", "cat"})
	if name != "image" || strings.Join(rest, " ") != "--or -m x cat" {
		t.Fatalf("%s %v", name, rest)
	}
}
