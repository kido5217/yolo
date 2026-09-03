// Package mockllm is the S8.1 deterministic OpenAI-compatible mock (spec
// §7.3; root AGENTS.md: unit tests never hit the network): it serves a
// pinned canned chat-completions stream with byte-deterministic SSE
// frames on a 127.0.0.1 listener; the S8.2 parity capture runs the
// upstream npm TUI against it. Pure net/http — no state, no egress.
package mockllm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// The fixed wire values (pinned for the byte-deterministic captures, D1).
const (
	completionID = "chatcmpl-canned01"
	bashCallID   = "call_canned1"
	todoCallID   = "call_canned2"
	fixedCreated = 1700000000
	fixedModel   = "canned"
)

// modelsJSON is the pinned GET /models body (byte-identical).
var modelsJSON = []byte(`{"object":"list","data":[{"id":"canned","object":"model","created":1700000000,"owned_by":"parity"}]}`)

var notFoundJSON = []byte(`{"error":{"message":"not found"}}`)

// Usage is the canned usage reported in the completion (the yolo
// llm.Usage int shape — the D1 cross-pin target).
type Usage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

// Tool is one canned tool call (name + the JSON args string the
// completion streams).
type Tool struct {
	Name string `json:"name"`
	Args string `json:"args"`
}

// Canned is one scripted turn: the text turn streams Reply as content
// chunks; a tool turn streams Tool as tool_calls frames, then — once the
// client posts the tool result — the ToolReply text turn.
type Canned struct {
	Prompt    string `json:"prompt"`
	Reply     string `json:"reply,omitempty"`
	ChunkSize int    `json:"chunk_size,omitempty"`
	Tool      *Tool  `json:"tool,omitempty"`
	ToolReply string `json:"tool_reply,omitempty"`
	Usage     Usage  `json:"usage"`
}

// Book is the canned set (the pinned fixture
// internal/tui/testdata/parity/canned.json decodes to exactly
// DefaultBook — TestCannedMatchesDefault).
type Book struct {
	Text Canned `json:"text"`
	Tool Canned `json:"tool"`
	Todo Canned `json:"todo"`
}

// DefaultBook is the built-in book (D1).
func DefaultBook() Book {
	return Book{
		Text: Canned{
			Prompt:    "say hi to the mock",
			Reply:     "## Heading\n\nSome **bold** and `inline code` text.\n\n- one\n- two\n\n> a quote\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n[link](https://example.com)\n\n```js\nconst x = 1;\n```\n\n你好 world\n\nDone.",
			ChunkSize: 6,
			Usage:     Usage{Input: 12, Output: 40},
		},
		Tool: Canned{
			Prompt:    "run the parity check",
			Tool:      &Tool{Name: "bash", Args: `{"command":"echo parity-ok"}`},
			ToolReply: "The check printed parity-ok.\n",
			ChunkSize: 6,
			Usage:     Usage{Input: 12, Output: 40},
		},
		Todo: Canned{
			Prompt:    "add two todos",
			Tool:      &Tool{Name: "todowrite", Args: `{"todos":[{"content":"first item","status":"in_progress"},{"content":"second item","status":"pending"}]}`},
			ToolReply: "Todos updated.\n",
			ChunkSize: 6,
			Usage:     Usage{Input: 12, Output: 40},
		},
	}
}

// LoadBook decodes a canned book (the S8.2 capture passes the shared
// fixture to the mock via -canned so both sides share one source).
func LoadBook(raw []byte) (Book, error) {
	var b Book
	if err := json.Unmarshal(raw, &b); err != nil {
		return Book{}, err
	}
	return b, nil
}

// Handler returns the mock's http.Handler for one canned turn (D1): POST
// any */chat/completions serves the canned stream (a tool turn streams the
// tool call until the request carries a tool result, then ToolReply); GET
// any */models serves the pinned model list.
func Handler(c Canned) http.Handler {
	if c.ChunkSize <= 0 {
		c.ChunkSize = 6
	}
	toolTurn := c.Tool != nil
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/models") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(modelsJSON)
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/chat/completions") {
			var req chatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			reply := c.Reply
			if toolTurn && req.hasToolResult() {
				reply = c.ToolReply
			}
			if !toolTurn || req.hasToolResult() {
				if req.Stream {
					writeTextStream(w, c, reply)
				} else {
					writeTextJSON(w, c, reply)
				}
				return
			}
			writeToolStream(w, c)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(notFoundJSON)
	})
}

type chatRequest struct {
	Stream   bool `json:"stream"`
	Messages []struct {
		Role      string `json:"role"`
		ToolCalls []any  `json:"tool_calls"`
	} `json:"messages"`
}

func (q chatRequest) hasToolResult() bool {
	for _, m := range q.Messages {
		if m.Role == "tool" || (m.Role == "assistant" && len(m.ToolCalls) > 0) {
			return true
		}
	}
	return false
}

// The wire shapes (field order = byte order — the determinism pin).
type wireUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (u Usage) wire() wireUsage {
	return wireUsage{PromptTokens: u.Input, CompletionTokens: u.Output, TotalTokens: u.Input + u.Output}
}

type wireFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type wireToolCall struct {
	Index    int          `json:"index"`
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function wireFunction `json:"function"`
}

type wireDelta struct {
	Role      string         `json:"role,omitempty"`
	Content   string         `json:"content,omitempty"`
	ToolCalls []wireToolCall `json:"tool_calls,omitempty"`
}

type wireChoice struct {
	Index        int       `json:"index"`
	Delta        wireDelta `json:"delta"`
	FinishReason *string   `json:"finish_reason,omitempty"`
}

type wireFrame struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []wireChoice `json:"choices"`
	Usage   *wireUsage   `json:"usage,omitempty"`
}

type wireNSMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type wireNSChoice struct {
	Index        int           `json:"index"`
	Message      wireNSMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type wireNSBody struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []wireNSChoice `json:"choices"`
	Usage   wireUsage      `json:"usage"`
}

func sse(w http.ResponseWriter, v any) {
	b, _ := json.Marshal(v)
	fmt.Fprintf(w, "data: %s\n\n", b)
}

func baseFrame() wireFrame {
	return wireFrame{ID: completionID, Object: "chat.completion.chunk", Created: fixedCreated, Model: fixedModel}
}

// callIDFor is the fixed tool-call id (one per tool name — D1).
func callIDFor(name string) string {
	if name == "todowrite" {
		return todoCallID
	}
	return bashCallID
}

// chunks splits s into rune runs of at most n runes (the D1 6-rune pin).
func chunks(s string, n int) []string {
	if n < 1 {
		n = 1
	}
	r := []rune(s)
	out := make([]string, 0, (len(r)+n-1)/n)
	for i := 0; i < len(r); i += n {
		end := i + n
		if end > len(r) {
			end = len(r)
		}
		out = append(out, string(r[i:end]))
	}
	return out
}

func writeTextStream(w http.ResponseWriter, c Canned, reply string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	f := baseFrame()
	f.Choices = []wireChoice{{Index: 0, Delta: wireDelta{Role: "assistant"}}}
	sse(w, f)
	for _, p := range chunks(reply, c.ChunkSize) {
		f := baseFrame()
		f.Choices = []wireChoice{{Index: 0, Delta: wireDelta{Content: p}}}
		sse(w, f)
	}
	f = baseFrame()
	stop := "stop"
	f.Choices = []wireChoice{{Index: 0, FinishReason: &stop}}
	sse(w, f)
	f = baseFrame()
	f.Choices = []wireChoice{}
	u := c.Usage.wire()
	f.Usage = &u
	sse(w, f)
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func writeToolStream(w http.ResponseWriter, c Canned) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	id := callIDFor(c.Tool.Name)
	f := baseFrame()
	f.Choices = []wireChoice{{Index: 0, Delta: wireDelta{Role: "assistant"}}}
	sse(w, f)
	f = baseFrame()
	f.Choices = []wireChoice{{Index: 0, Delta: wireDelta{ToolCalls: []wireToolCall{{Index: 0, ID: id, Type: "function", Function: wireFunction{Name: c.Tool.Name}}}}}}
	sse(w, f)
	f = baseFrame()
	f.Choices = []wireChoice{{Index: 0, Delta: wireDelta{ToolCalls: []wireToolCall{{Index: 0, Function: wireFunction{Arguments: c.Tool.Args}}}}}}
	sse(w, f)
	f = baseFrame()
	toolCalls := "tool_calls"
	f.Choices = []wireChoice{{Index: 0, FinishReason: &toolCalls}}
	sse(w, f)
	f = baseFrame()
	f.Choices = []wireChoice{}
	u := c.Usage.wire()
	f.Usage = &u
	sse(w, f)
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func writeTextJSON(w http.ResponseWriter, c Canned, reply string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	body := wireNSBody{
		ID:      completionID,
		Object:  "chat.completion",
		Created: fixedCreated,
		Model:   fixedModel,
		Choices: []wireNSChoice{{Index: 0, Message: wireNSMessage{Role: "assistant", Content: reply}, FinishReason: "stop"}},
		Usage:   c.Usage.wire(),
	}
	_ = json.NewEncoder(w).Encode(body)
}
