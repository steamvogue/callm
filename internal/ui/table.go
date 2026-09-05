package ui

import (
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"

	"callm/internal/client"
)

func parsePricePerMillion(priceVal interface{}) string {
	if priceVal == nil {
		return "unknown"
	}
	var f float64
	switch v := priceVal.(type) {
	case float64:
		f = v
	case float32:
		f = float64(v)
	case string:
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			f = parsed
		} else {
			return "unknown"
		}
	default:
		return "unknown"
	}
	if math.IsNaN(f) || math.IsInf(f, 0) || f < 0 {
		return "unknown"
	}
	perMtok := f * 1000000
	if perMtok == 0 {
		return "0"
	}
	return strconv.FormatFloat(perMtok, 'f', -1, 64)
}

// PrintModelsTable prints a formatted table of models matching the filter.
func PrintModelsTable(out io.Writer, models []client.ModelInfo, filter string) error {
	var re *regexp.Regexp
	var err error
	if filter != "" {
		re, err = regexp.Compile("(?i)" + filter)
		if err != nil {
			return fmt.Errorf("invalid filter regex '%s': %w", filter, err)
		}
	}

	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "MODEL\tCONTEXT\t$/Mtok-IN\t$/Mtok-OUT\tMODALITIES")

	for _, m := range models {
		if re != nil && !re.MatchString(m.ID) && !re.MatchString(m.CanonicalSlug) {
			continue
		}

		priceIn := "unknown"
		priceOut := "unknown"
		if m.Pricing != nil {
			priceIn = parsePricePerMillion(m.Pricing.Prompt)
			priceOut = parsePricePerMillion(m.Pricing.Completion)
		}

		modalities := "text"
		if m.Architecture != nil && len(m.Architecture.InputModalities) > 0 {
			modalities = strings.Join(m.Architecture.InputModalities, ",")
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			m.ID, formatContext(m.ContextLength), priceIn, priceOut, modalities)
	}

	return w.Flush()
}

// PrintModelInfo displays detailed specifications for a single model.
func PrintModelInfo(out io.Writer, m client.ModelInfo) {
	priceIn := "unknown"
	priceOut := "unknown"
	cacheRead := "unknown"
	cacheWrite := "unknown"
	if m.Pricing != nil {
		priceIn = parsePricePerMillion(m.Pricing.Prompt)
		priceOut = parsePricePerMillion(m.Pricing.Completion)
		cacheRead = parsePricePerMillion(m.Pricing.InputCacheRead)
		cacheWrite = parsePricePerMillion(m.Pricing.InputCacheWrite)
	}

	modality := "text->text"
	inputs := "text"
	outputs := "text"
	if m.Architecture != nil {
		if m.Architecture.Modality != "" {
			modality = m.Architecture.Modality
		}
		if len(m.Architecture.InputModalities) > 0 {
			inputs = strings.Join(m.Architecture.InputModalities, ", ")
		}
		if len(m.Architecture.OutputModalities) > 0 {
			outputs = strings.Join(m.Architecture.OutputModalities, ", ")
		}
	}

	slug := m.CanonicalSlug
	if slug == "" {
		slug = m.ID
	}

	supported := strings.Join(m.SupportedParameters, ", ")
	if supported == "" {
		supported = "none specified"
	}

	fmt.Fprintf(out, "Model ID:         %s\n", m.ID)
	fmt.Fprintf(out, "Canonical Slug:   %s\n", slug)
	fmt.Fprintf(out, "Context Length:   %s\n", formatContext(m.ContextLength))
	fmt.Fprintf(out, "Modality:         %s\n", modality)
	fmt.Fprintf(out, "Input Modalities: %s\n", inputs)
	fmt.Fprintf(out, "Output Modalities:%s\n", outputs)
	fmt.Fprintf(out, "Pricing (Prompt): $%s/Mtok\n", priceIn)
	fmt.Fprintf(out, "Pricing (Comp):   $%s/Mtok\n", priceOut)
	fmt.Fprintf(out, "Cache Read:       $%s/Mtok\n", cacheRead)
	fmt.Fprintf(out, "Cache Write:      $%s/Mtok\n", cacheWrite)
	fmt.Fprintf(out, "Supported Params: %s\n", supported)
}

func formatContext(n int64) string {
	if n <= 0 {
		return "unknown"
	}
	return strconv.FormatInt(n, 10)
}
