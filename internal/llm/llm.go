// Package llm defines the provider-agnostic streaming chat interface.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

// Role is a chat message role.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall is one assistant-invoked tool call.
type ToolCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"` // raw JSON object
}

// Message is one chat message.
type Message struct {
	Role       Role
	Content    string
	ToolCallID string     // RoleTool
	ToolCalls  []ToolCall // RoleAssistant
}

// ToolDef describes a callable tool for the model.
type ToolDef struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// Usage is token accounting for a stream.
type Usage struct {
	Input, Output, Reasoning, CacheRead, CacheWrite int
}

// Request is one streaming chat completion request.
type Request struct {
	Model       string
	APIKey      string
	BaseURL     string
	Messages    []Message
	Tools       []ToolDef
	Temperature *float64
	MaxTokens   int
}

// Part is one emitted stream piece, in stream order.
type Part struct {
	Kind   string // "text" | "reasoning" | "tool"
	Name   string // tool name (Kind=="tool")
	CallID string // stable per tool (Kind=="tool")
	Args   json.RawMessage
	Text   string // delta payload
	Usage  *Usage // on the final part of the stream
	Finish string // "stop" | "tool_calls" | "length" | "error" (final part)
	Err    error  // non-nil only on the final part after a 200 began
}

// PartStream delivers Parts.
type PartStream struct {
	Parts <-chan Part
}

// Next blocks for the next part. Error only when ctx is done (ctx.Err), or
// after the final part was delivered (io.EOF).
func (s PartStream) Next(ctx context.Context) (Part, error) {
	select {
	case <-ctx.Done():
		return Part{}, ctx.Err()
	case p, ok := <-s.Parts:
		if !ok {
			return Part{}, io.EOF
		}
		return p, nil
	}
}

// Driver streams chat completions.
type Driver interface {
	Stream(ctx context.Context, req Request) (PartStream, error)
}

// OpenAI implements Driver over OpenAI-compatible chat/completions SSE.
type OpenAI struct{ Client *http.Client }

// NewOpenAI returns an OpenAI driver.
func NewOpenAI(c *http.Client) Driver { return &OpenAI{Client: c} }

// TransientError wraps a retryable upstream failure with its HTTP status.
type TransientError struct {
	Status int
	Err    error
}

func (e *TransientError) Error() string { return e.Err.Error() }
func (e *TransientError) Unwrap() error { return e.Err }

// APIError is a decoded non-2xx upstream response. Body is the drained
// (capped, 64 KiB) response body; Message is the provider-decoded error text
// ("" when the body carries no message). The error text keeps the existing
// "upstream error (http %d): ..." framing so pins and logs stay comparable.
type APIError struct {
	Status  int
	Body    []byte
	Message string
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = strings.TrimSpace(string(e.Body))
	}
	if msg == "" {
		msg = fmt.Sprintf("%d %s", e.Status, http.StatusText(e.Status))
	}
	return fmt.Sprintf("upstream error (http %d): %s", e.Status, msg)
}

// IsTransient reports whether err is retryable: a 429/5xx TransientError or a
// network error. Context errors are not transient.
func IsTransient(err error) bool {
	var te *TransientError
	if errors.As(err, &te) {
		return te.Status == 429 || te.Status >= 500
	}
	var ne net.Error
	return errors.As(err, &ne)
}
