package ui

import (
	"testing"

	"callm/internal/client"
)

func TestFilterModels(t *testing.T) {
	models := []client.ModelInfo{
		{ID: "deepseek/deepseek-v4-flash", CanonicalSlug: "deepseek/deepseek-v4-flash"},
		{ID: "z-ai/glm-5.3", Name: "GLM 5.3"},
		{ID: "qwen/qwen3.8-max", DisplayName: "Qwen 3.8 Max"},
		{ID: "moonshotai/kimi-k3"},
	}
	for _, tc := range []struct {
		name  string
		regex string
		terms []string
		want  []string
	}{
		{"normalized dot", "", []string{"z.ai"}, []string{"z-ai/glm-5.3"}},
		{"comma terms", "", []string{"deepseek,qwen"}, []string{"deepseek/deepseek-v4-flash", "qwen/qwen3.8-max"}},
		{"display name", "", []string{"glm 5"}, []string{"z-ai/glm-5.3"}},
		{"regex and terms", "^z-ai", []string{"glm"}, []string{"z-ai/glm-5.3"}},
		{"regex only", "deepseek|kimi", nil, []string{"deepseek/deepseek-v4-flash", "moonshotai/kimi-k3"}},
		{"separator terms", "", []string{"z_ai", "z.ai"}, []string{"z-ai/glm-5.3"}},
		{"unused terms", "", []string{" , ,"}, []string{"deepseek/deepseek-v4-flash", "z-ai/glm-5.3", "qwen/qwen3.8-max", "moonshotai/kimi-k3"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FilterModels(models, tc.regex, tc.terms)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d models: %+v", len(got), got)
			}
			for i, id := range tc.want {
				if got[i].ID != id {
					t.Fatalf("got[%d]=%s want %s", i, got[i].ID, id)
				}
			}
		})
	}
}

func TestFilterModelsInvalidRegex(t *testing.T) {
	if _, err := FilterModels(nil, "([", nil); err == nil {
		t.Fatal("invalid regex accepted")
	}
}
