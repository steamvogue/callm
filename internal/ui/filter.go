package ui

import (
	"fmt"
	"regexp"
	"strings"

	"callm/internal/client"
)

// normalizeFilter folds separators so z.ai, z-ai, and z_ai compare equal.
func normalizeFilter(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	lastDash := false
	for _, r := range strings.ToLower(value) {
		switch r {
		case '-', '_', '.', '/', ' ', ':', '\\':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		default:
			b.WriteRune(r)
			lastDash = false
		}
	}
	return strings.Trim(b.String(), "-")
}

// normalizeFilterTerms splits comma-separated values, trims them, and drops
// entries that normalize to nothing.
func normalizeFilterTerms(values []string) []string {
	var terms []string
	for _, value := range values {
		for _, term := range strings.Split(value, ",") {
			if normalized := normalizeFilter(term); normalized != "" {
				terms = append(terms, normalized)
			}
		}
	}
	return terms
}

func matchesFilterTerms(model client.ModelInfo, terms []string) bool {
	fields := []string{
		normalizeFilter(model.ID),
		normalizeFilter(model.CanonicalSlug),
		normalizeFilter(model.Name),
		normalizeFilter(model.DisplayName),
	}
	for _, term := range terms {
		for _, field := range fields {
			if field != "" && strings.Contains(field, term) {
				return true
			}
		}
	}
	return false
}

// FilterModels returns models matching an optional case-insensitive regex on
// IDs/slugs and optional normalized substring terms. Terms are OR-combined;
// the regex and the terms are AND-combined.
func FilterModels(models []client.ModelInfo, regexFilter string, filterValues []string) ([]client.ModelInfo, error) {
	var re *regexp.Regexp
	var err error
	if regexFilter != "" {
		re, err = regexp.Compile("(?i)" + regexFilter)
		if err != nil {
			return nil, fmt.Errorf("invalid filter regex '%s': %w", regexFilter, err)
		}
	}
	terms := normalizeFilterTerms(filterValues)
	if re == nil && len(terms) == 0 {
		return models, nil
	}

	filtered := make([]client.ModelInfo, 0, len(models))
	for _, model := range models {
		if re != nil && !re.MatchString(model.ID) && !re.MatchString(model.CanonicalSlug) {
			continue
		}
		if len(terms) > 0 && !matchesFilterTerms(model, terms) {
			continue
		}
		filtered = append(filtered, model)
	}
	return filtered, nil
}
