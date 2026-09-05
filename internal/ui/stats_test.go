package ui

import (
	"bytes"
	"callm/internal/client"
	"strings"
	"testing"
	"time"
)

func TestUnknownMetadataAndStringCost(t *testing.T) {
	var out bytes.Buffer
	PrintModelsTable(&out, []client.ModelInfo{{ID: "a"}}, "")
	if strings.Count(out.String(), "unknown") < 3 {
		t.Fatal(out.String())
	}
	out.Reset()
	PrintStats(&out, time.Second, nil, "a")
	if !strings.Contains(out.String(), "usage unavailable") || strings.Contains(out.String(), "0 tokens") {
		t.Fatal(out.String())
	}
	out.Reset()
	PrintStats(&out, time.Second, &client.Usage{Cost: "0.125"}, "a")
	if !strings.Contains(out.String(), "$0.125000") {
		t.Fatal(out.String())
	}
	if parsePricePerMillion("0") != "0" || parsePricePerMillion("bad") != "unknown" {
		t.Fatal("price decoding")
	}
}
