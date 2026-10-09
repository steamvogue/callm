package pipeline

import (
	"callm/internal/client"
	"testing"
	"time"
)

func TestSchemaValidation(t *testing.T) {
	s, err := CompileSchema([]byte(`{"type":"object","properties":{"email":{"type":"string","format":"email"},"n":{"$ref":"#/$defs/count"}},"required":["email","n"],"additionalProperties":false,"$defs":{"count":{"type":"integer","minimum":2}}}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		answer string
		valid  bool
	}{
		{`{"email":"a@example.com","n":2}`, true},
		{`{"email":"bad","n":2}`, false},
		{`{"email":"a@example.com","n":1}`, false},
		{`{"email":"a@example.com","n":2,"extra":true}`, false},
		{`{"email":"a@example.com"}`, false},
		{"```json\n{}\n```", false},
		{`{} {}`, false},
	} {
		if err := s.Validate(tc.answer); (err == nil) != tc.valid {
			t.Fatalf("%s: %v", tc.answer, err)
		}
	}
}

func TestSchemaCompilationRejectsInvalidAndExternal(t *testing.T) {
	for _, schema := range []string{`{"type":17}`, `{} {}`, `{"$ref":"https://example.com/schema.json"}`, `{"$ref":"file:///etc/passwd"}`, `{"$ref":"other.json"}`} {
		if _, err := CompileSchema([]byte(schema)); err == nil {
			t.Fatalf("accepted %s", schema)
		}
	}
	for _, schema := range []string{`true`, `false`, `{"$schema":"http://json-schema.org/draft-07/schema#","definitions":{"s":{"type":"string"}},"$ref":"#/definitions/s"}`} {
		if _, err := CompileSchema([]byte(schema)); err != nil {
			t.Fatalf("%s: %v", schema, err)
		}
	}
}

func TestResultDistinguishesCompletionAndSchemaFailures(t *testing.T) {
	s, _ := CompileSchema([]byte(`{"type":"object","required":["n"],"properties":{"n":{"type":"integer"}}}`))
	for _, tc := range []struct{ answer, finish, code string }{
		{`{"n":2}`, "stop", ""},
		{`{"n":"two"}`, "stop", "invalid_schema_output"},
		{`{"n":2}`, "length", "invalid_completion"},
	} {
		resp := &client.ChatResponse{Model: "returned", Choices: []client.ChatChoice{{Message: client.RespMsg{Content: tc.answer}, FinishReason: tc.finish}}}
		r := NewResult("requested", resp, 2*time.Second, s, nil)
		if r.Version != 1 || r.DurationMS != 2000 || r.RequestedModel != "requested" || r.ReturnedModel != "returned" || r.Usage != nil {
			t.Fatalf("metadata: %+v", r)
		}
		if tc.code == "" {
			if r.Status != "ok" || r.SchemaValid == nil || !*r.SchemaValid {
				t.Fatalf("%+v", r)
			}
		} else if r.Status != "error" || r.Error == nil || r.Error.Code != tc.code {
			t.Fatalf("%+v", r)
		}
	}
}
