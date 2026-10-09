package client

import "strings"

func permitsEffort(effort string, allowed []string) bool {
	if effort == "" {
		return true
	}
	for _, value := range allowed {
		if value == effort {
			return true
		}
	}
	return false
}

var legacyEfforts = []string{"low", "medium", "high"}

// Known model profiles are deliberately bounded. An unrecognized future/custom
// name does not inherit capabilities solely from a vendor prefix.
func knownModel(model string, base string) bool {
	if model == base {
		return true
	}
	suffix := strings.TrimPrefix(model, base+"-")
	if suffix == model {
		return false
	}
	// Claude/OpenAI dated snapshots use either YYYYMMDD or YYYY-MM-DD.
	digits := strings.ReplaceAll(suffix, "-", "")
	if len(digits) != 8 {
		return false
	}
	for _, d := range digits {
		if d < '0' || d > '9' {
			return false
		}
	}
	return true
}

type claudeProfile struct {
	adaptive, manual, fixedSampling bool
	efforts                         []string
}

func nativeClaudeProfile(model string) claudeProfile {
	for _, base := range []string{"claude-sonnet-4-6", "claude-opus-4-6"} {
		if knownModel(model, base) {
			return claudeProfile{adaptive: true, manual: true, efforts: []string{"low", "medium", "high", "max"}}
		}
	}
	for _, base := range []string{"claude-sonnet-5", "claude-sonnet-5-5", "claude-haiku-5-5", "claude-opus-4-7", "claude-opus-4-8", "claude-opus-5", "claude-opus-5-5", "claude-fable-5", "claude-fable-5-1", "claude-mythos-5", "claude-mythos-5-1"} {
		if knownModel(model, base) {
			return claudeProfile{adaptive: true, fixedSampling: true, efforts: []string{"low", "medium", "high", "xhigh", "max"}}
		}
	}
	return claudeProfile{manual: true, efforts: legacyEfforts}
}

type openAIProfile struct {
	reasoning            bool
	efforts              []string
	samplingRequiresNone bool
}

func openAIModelProfile(model string) openAIProfile {
	model = strings.TrimPrefix(model, "openai/")
	for _, base := range []string{"gpt-6-astra", "gpt-6.1-sol"} {
		if knownModel(model, base) {
			return openAIProfile{reasoning: true, samplingRequiresNone: true, efforts: []string{"low", "medium", "high", "xhigh", "max"}}
		}
	}
	for _, base := range []string{"gpt-6-sol", "gpt-6-luna"} {
		if knownModel(model, base) {
			return openAIProfile{reasoning: true, samplingRequiresNone: true, efforts: []string{"none", "low", "medium", "high", "xhigh", "max"}}
		}
	}
	for _, prefix := range []string{"o1", "o3", "o4"} {
		if model == prefix || strings.HasPrefix(model, prefix+"-") {
			return openAIProfile{reasoning: true, efforts: legacyEfforts}
		}
	}
	return openAIProfile{efforts: legacyEfforts}
}
