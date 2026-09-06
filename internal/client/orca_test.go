package client

import (
	"context"
	"encoding/json"
	"testing"
)

func TestOrcaCost(t *testing.T) {
	for _, tc := range []struct {
		data  string
		want  float64
		valid bool
	}{
		{`{"cost_usd":0.125}`, 0.125, true}, {`{"cost_usd":"0.125"}`, 0.125, true},
		{`{"cost_usd":0}`, 0, true}, {`{}`, 0, false}, {`{"cost_usd":null}`, 0, false},
		{`{"cost_usd":-1}`, 0, false}, {`{"cost_usd":"NaN"}`, 0, false},
		{`{"cost_usd":"Inf"}`, 0, false}, {`{"cost_usd":"bad"}`, 0, false},
		{`{"cost":0,"cost_usd":1}`, 0, true},
	} {
		var u Usage
		if err := json.Unmarshal([]byte(tc.data), &u); err != nil {
			t.Fatal(err)
		}
		if got, ok := u.CostValue(); got != tc.want || ok != tc.valid {
			t.Errorf("%s => %v %v", tc.data, got, ok)
		}
	}
}

func TestOrcaDetectionAndCostHeader(t *testing.T) {
	for _, tc := range []struct {
		url, explicit string
		orca          bool
	}{
		{"https://api.orcarouter.ai/v1", "", true},
		{"https://api.orcarouter.ai.evil.invalid/v1", "", false},
		{"https://proxy.invalid/v1", "orca", true},
		{"https://api.orcarouter.ai/v1", "or", false},
	} {
		c := NewClient(tc.url, "dummy", tc.explicit)
		c.IncludeCost = true
		req, err := c.request(context.Background(), "POST", "/chat/completions", []byte(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		if (req.Header.Get("X-OrcaRouter-Include-Cost") == "true") != tc.orca {
			t.Fatal("cost header provider isolation")
		}
	}
}
