package pipeline

import (
	"errors"
	"time"

	"callm/internal/client"
)

// Result is versioned separately from the original provider JSON (--json).
// An answer in an error result may be partial and must not be published.
type Result struct {
	Version        int           `json:"version"`
	Status         string        `json:"status"`
	RequestedModel string        `json:"requested_model"`
	ReturnedModel  string        `json:"returned_model"`
	Answer         string        `json:"answer"`
	FinishReason   string        `json:"finish_reason"`
	Refusal        string        `json:"refusal"`
	Usage          *client.Usage `json:"usage"`
	DurationMS     int64         `json:"duration_ms"`
	SchemaValid    *bool         `json:"schema_valid"`
	Error          *ResultError  `json:"error"`
}

type ResultError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewResult(requested string, response *client.ChatResponse, duration time.Duration, schema *Schema, requestErr error) Result {
	r := Result{Version: 1, Status: "ok", RequestedModel: requested, DurationMS: duration.Milliseconds()}
	err := requestErr
	code := "request_failed"
	if response != nil {
		r.ReturnedModel, r.Usage = response.Model, response.Usage
		if len(response.Choices) > 0 {
			choice := response.Choices[0]
			r.Answer, r.FinishReason, r.Refusal = choice.Message.Content, choice.FinishReason, choice.Message.Refusal
			if err == nil {
				err, code = choice.Completion().Validate(true, false, false), "invalid_completion"
			}
		} else if err == nil {
			err, code = errors.New("response has no completion choices"), "invalid_completion"
		}
	} else if err == nil {
		err = errors.New("missing response")
	}
	if err == nil && schema != nil {
		err, code = schema.Validate(r.Answer), "invalid_schema_output"
		valid := err == nil
		r.SchemaValid = &valid
	}
	if err != nil {
		r.Status = "error"
		r.Error = &ResultError{Code: code, Message: err.Error()}
	}
	return r
}
