// Package pipeline provides offline validation and stable results for CLI pipelines.
package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const MaxSchemaBytes = 1 << 20

type offlineLoader struct{}

func (offlineLoader) Load(url string) (any, error) {
	return nil, fmt.Errorf("external schema references are disabled: %s (use in-document $defs)", url)
}

type Schema struct {
	Raw      json.RawMessage
	compiled *jsonschema.Schema
}

func CompileSchema(data []byte) (*Schema, error) {
	if len(data) > MaxSchemaBytes {
		return nil, fmt.Errorf("schema exceeds 1 MiB")
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("invalid schema JSON: %w", err)
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	c.UseLoader(offlineLoader{})
	const resource = "https://callm.invalid/output-schema.json"
	if err := c.AddResource(resource, doc); err != nil {
		return nil, err
	}
	compiled, err := c.Compile(resource)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON Schema: %w", err)
	}
	return &Schema{Raw: append(json.RawMessage(nil), data...), compiled: compiled}, nil
}

func (s *Schema) Validate(answer string) error {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewBufferString(answer))
	if err != nil {
		return fmt.Errorf("answer is not a single JSON value: %w", err)
	}
	if err := s.compiled.Validate(doc); err != nil {
		return fmt.Errorf("answer does not match schema: %w", err)
	}
	return nil
}
