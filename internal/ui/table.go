package ui

import (
	"errors"
	"fmt"
	"io"
	"math"
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
	models, err := FilterModels(models, filter, nil)
	if err != nil {
		return err
	}

	checked := &checkedWriter{Writer: out}
	w := tabwriter.NewWriter(checked, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "MODEL\tCONTEXT\t$/Mtok-IN\t$/Mtok-OUT\tMODALITIES")

	for _, m := range models {
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

	return errors.Join(checked.err, w.Flush())
}

// PrintModelInfo displays detailed specifications for a single model.
func PrintModelInfo(out io.Writer, m client.ModelInfo) error {
	var text strings.Builder
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

	fmt.Fprintf(&text, "Model ID:         %s\n", m.ID)
	fmt.Fprintf(&text, "Canonical Slug:   %s\n", slug)
	fmt.Fprintf(&text, "Context Length:   %s\n", formatContext(m.ContextLength))
	fmt.Fprintf(&text, "Modality:         %s\n", modality)
	fmt.Fprintf(&text, "Input Modalities: %s\n", inputs)
	fmt.Fprintf(&text, "Output Modalities:%s\n", outputs)
	fmt.Fprintf(&text, "Pricing (Prompt): $%s/Mtok\n", priceIn)
	fmt.Fprintf(&text, "Pricing (Comp):   $%s/Mtok\n", priceOut)
	fmt.Fprintf(&text, "Cache Read:       $%s/Mtok\n", cacheRead)
	fmt.Fprintf(&text, "Cache Write:      $%s/Mtok\n", cacheWrite)
	fmt.Fprintf(&text, "Supported Params: %s\n", supported)
	return writeText(out, text.String())
}

type checkedWriter struct {
	io.Writer
	err error
}

func (w *checkedWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.Writer.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	w.err = err
	return n, err
}

func formatContext(n int64) string {
	if n <= 0 {
		return "unknown"
	}
	return strconv.FormatInt(n, 10)
}
