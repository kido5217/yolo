package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// Anthropic implements Driver over the Anthropic Messages SSE API.
type Anthropic struct{ Client *http.Client }

// NewAnthropic returns an Anthropic driver.
func NewAnthropic(c *http.Client) Driver { return &Anthropic{Client: c} }

// Stream POSTs {BaseURL}/messages and parses the SSE stream.
func (a *Anthropic) Stream(ctx context.Context, req Request) (PartStream, error) {
	body, err := json.Marshal(anRequest(req))
	if err != nil {
		return PartStream{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(req.BaseURL, "/")+"/messages", bytes.NewReader(body))
	if err != nil {
		return PartStream{}, err
	}
	httpReq.Header.Set("x-api-key", req.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := a.Client.Do(httpReq)
	if err != nil {
		return PartStream{}, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return PartStream{}, oaUpstreamError(resp) // same {"error":{...}} envelope handling
	}
	ch := make(chan Part, 64)
	go a.anReadSSE(ctx, resp.Body, ch)
	return PartStream{Parts: ch}, nil
}

// wire request types

type anBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type anMsg struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

func anRequest(req Request) map[string]any {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}
	out := map[string]any{
		"model":      req.Model,
		"stream":     true,
		"max_tokens": maxTokens,
	}
	if req.Temperature != nil {
		out["temperature"] = *req.Temperature
	}
	if len(req.Tools) > 0 {
		tools := []map[string]any{}
		for _, td := range req.Tools {
			t := map[string]any{"name": td.Name, "input_schema": json.RawMessage(td.Parameters)}
			if td.Description != "" {
				t["description"] = td.Description
			}
			tools = append(tools, t)
		}
		out["tools"] = tools
	}
	var system []string
	var msgs []anMsg
	for _, m := range req.Messages {
		switch m.Role {
		case RoleSystem:
			system = append(system, m.Content)
		case RoleTool:
			b := anBlock{Type: "tool_result", ToolUseID: m.ToolCallID, Content: m.Content}
			msgs = append(msgs, anMsg{Role: "user", Content: []anBlock{b}})
		case RoleAssistant:
			if len(m.ToolCalls) > 0 {
				blocks := []anBlock{}
				if m.Content != "" {
					blocks = append(blocks, anBlock{Type: "text", Text: m.Content})
				}
				for _, tc := range m.ToolCalls {
					in := json.RawMessage("{}")
					if len(tc.Args) > 0 {
						in = json.RawMessage(tc.Args)
					}
					blocks = append(blocks, anBlock{Type: "tool_use", ID: tc.ID, Name: tc.Name, Input: in})
				}
				msgs = append(msgs, anMsg{Role: "assistant", Content: blocks})
			} else {
				msgs = append(msgs, anMsg{Role: "assistant", Content: m.Content})
			}
		default:
			msgs = append(msgs, anMsg{Role: string(m.Role), Content: m.Content})
		}
	}
	out["messages"] = msgs
	if len(system) > 0 {
		out["system"] = strings.Join(system, "\n\n")
	}
	return out
}

// event frames

type anEv struct {
	Type    string `json:"type"`
	Message struct {
		Usage struct {
			InputTokens int `json:"input_tokens"`
		} `json:"usage"`
	} `json:"message"`
	Index        int `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"content_block"`
	Delta struct {
		Type        string  `json:"type"`
		Text        string  `json:"text"`
		Thinking    string  `json:"thinking"`
		PartialJSON string  `json:"partial_json"`
		StopReason  *string `json:"stop_reason"`
	} `json:"delta"`
	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

type anAccum struct {
	kind string // "text" | "thinking" | "tool_use"
	id   string
	name string
	args strings.Builder
}

func (a *Anthropic) anReadSSE(ctx context.Context, body io.ReadCloser, ch chan Part) {
	defer close(ch)
	defer body.Close()
	send := func(p Part) {
		select {
		case ch <- p:
		case <-ctx.Done():
		}
	}

	blocks := map[int]*anAccum{}
	inputTokens, outputTokens := 0, 0
	cacheRead := 0
	pendingFinish := ""
	streamErr, readErr := error(nil), error(nil)
	finished := false
	finish := func() {
		if finished {
			return
		}
		finished = true
		u := &Usage{Input: inputTokens, Output: outputTokens, CacheRead: cacheRead}
		switch {
		case streamErr != nil:
			send(Part{Kind: "text", Finish: "error", Err: streamErr})
		case pendingFinish != "":
			send(Part{Kind: "text", Finish: pendingFinish, Usage: u})
		case readErr != nil:
			send(Part{Kind: "text", Finish: "error", Err: readErr})
		default:
			send(Part{Kind: "text", Finish: "error", Err: errors.New("stream ended without message_stop")})
		}
	}

	process := func(payload []byte) {
		var ev anEv
		if err := json.Unmarshal(payload, &ev); err != nil {
			return // ignore malformed frame
		}
		switch ev.Type {
		case "message_start":
			inputTokens = ev.Message.Usage.InputTokens
		case "content_block_start":
			blocks[ev.Index] = &anAccum{kind: ev.ContentBlock.Type, id: ev.ContentBlock.ID, name: ev.ContentBlock.Name}
		case "content_block_delta":
			b := blocks[ev.Index]
			switch ev.Delta.Type {
			case "text_delta":
				send(Part{Kind: "text", Text: ev.Delta.Text})
			case "thinking_delta":
				send(Part{Kind: "reasoning", Text: ev.Delta.Thinking})
			case "input_json_delta":
				if b != nil {
					b.args.WriteString(ev.Delta.PartialJSON)
				}
			}
		case "content_block_stop":
			if b, ok := blocks[ev.Index]; ok && b.kind == "tool_use" {
				args := b.args.String()
				if args == "" {
					args = "{}"
				}
				send(Part{Kind: "tool", CallID: b.id, Name: b.name, Args: json.RawMessage(args)})
			}
		case "message_delta":
			if ev.Delta.StopReason != nil && *ev.Delta.StopReason != "" {
				pendingFinish = anFinish(*ev.Delta.StopReason)
			}
			outputTokens = ev.Usage.OutputTokens
		case "message_stop":
			finish()
		case "error":
			msg := ev.Error.Message
			if msg == "" {
				msg = "stream error"
			}
			streamErr = errors.New(msg)
		}
	}

	// Byte-based line reading: the payload is assembled as []byte and
	// handed to json.Unmarshal directly, with the same parse semantics as
	// the string join (per-value trim, multi-data join with '\n').
	rd := bufio.NewReader(body)
	var dataVals [][]byte
	for {
		line, err := rd.ReadBytes('\n')
		if len(line) > 0 {
			line = bytes.TrimRight(line, "\r\n")
			switch {
			case len(line) == 0:
				if len(dataVals) > 0 {
					process(joinDataLines(dataVals))
					dataVals = nil
				}
			case bytes.HasPrefix(line, sseDataPrefix):
				dataVals = append(dataVals, bytes.TrimSpace(line[len(sseDataPrefix):]))
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				readErr = err
			}
			break
		}
		if finished {
			break
		}
	}
	finish()
}

func anFinish(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "tool_use":
		return "tool_calls"
	case "max_tokens":
		return "length"
	default:
		return reason
	}
}
