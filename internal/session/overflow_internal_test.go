package session

import (
	"errors"
	"testing"

	"github.com/kido5217/yolo/internal/llm"
)

// TestIsOverflowClassifier pins the ⑦ port of the upstream curated
// classifier (provider-error.ts patterns/exclusions + error.ts
// parseAPICallError): pattern hits and the status/body rules are overflow;
// exclusions, auth/quota/rate errors and generic text are NOT.
func TestIsOverflowClassifier(t *testing.T) {
	apiErr := func(status int, body, message string) error {
		return &llm.APIError{Status: status, Body: []byte(body), Message: message}
	}
	tests := []struct {
		name string
		err  error
		want bool
	}{
		// pattern hits (decoded provider text, ④'s output)
		{"prompt too long", errors.New("upstream error (http 400): prompt is too long: 120000 > 100000"), true},
		{
			"max context length digits",
			apiErr(400, `{"error":{"message":"Maximum context length is 128000 tokens"}}`,
				"Maximum context length is 128000 tokens"), true,
		},
		{"exceeds limit", apiErr(400, ``, "request exceeds the limit of 1000 tokens"), true},
		{
			"model context window exceeded code",
			apiErr(400, `{"error":{"code":"model_context_window_exceeded","message":"x"}}`, "x"), true,
		},
		{"request entity too large", errors.New("upstream error (http 413): Request entity too large"), true},
		{
			"prompt has tokens configured size",
			errors.New("prompt has 150,000 tokens, but the configured context size is 128,000 tokens"), true,
		},
		// status/body rules
		{"413 empty body", apiErr(413, ``, ""), true},
		{"400 empty body", apiErr(400, ``, ""), true},
		{
			"400 context_length_exceeded code",
			apiErr(400, `{"error":{"code":"context_length_exceeded","message":"Input exceeds context window of this model"}}`,
				"Input exceeds context window of this model"), true,
		},
		{"synthesized no-body form", errors.New("400 status code (no body)"), true},
		// exclusions (AND-NOT wins over pattern hits)
		{
			"rate limit",
			apiErr(429, `{"error":{"message":"Rate limit reached: you have used 128000 tokens per minute"}}`,
				"Rate limit reached: you have used 128000 tokens per minute"), false,
		},
		{"too many requests", apiErr(429, ``, "Too Many Requests"), false},
		{"throttling prefix", errors.New("throttling error: capacity exceeded"), false},
		{"service unavailable prefix", errors.New("service unavailable: please retry"), false},
		// real errors that the OLD regex mistook for overflow
		{
			"401 invalid key",
			apiErr(401, `{"error":{"message":"Invalid API key provided"}}`, "Invalid API key provided"), false,
		},
		{"403 forbidden", apiErr(403, ``, "Forbidden"), false},
		{"context canceled", errors.New("context canceled"), false},
		{
			"model not found",
			apiErr(404, `{"error":{"message":"model not found, no context"}}`, "model not found, no context"), false,
		},
		{"quota", apiErr(400, `{"error":{"code":"insufficient_quota","message":"quota exceeded"}}`, "quota exceeded"), false},
		{"nil", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isOverflowError(tc.err); got != tc.want {
				t.Fatalf("isOverflowError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
