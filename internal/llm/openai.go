package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Stream POSTs {BaseURL}/chat/completions and parses the SSE stream.
func (o *OpenAI) Stream(ctx context.Context, req Request) (PartStream, error) {
	body, err := json.Marshal(oaRequest(req))
	if err != nil {
		return PartStream{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(req.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return PartStream{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := o.Client.Do(httpReq)
	if err != nil {
		return PartStream{}, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return PartStream{}, oaUpstreamError(resp)
	}
	ch := make(chan Part, 64)
	go o.oaReadSSE(ctx, resp.Body, ch)
	return PartStream{Parts: ch}, nil
}

// oaUpstreamError builds an error from a non-2xx response (body ≤ 4KB drained;
// 429/5xx wrapped in *TransientError); the message prefers {"error":{...}}.
func oaUpstreamError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var err error
	var envelope struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(data, &envelope) == nil && len(envelope.Error) > 0 {
		var obj struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(envelope.Error, &obj) == nil && obj.Message != "" {
			err = errors.New(obj.Message)
		} else {
			var plain string
			if json.Unmarshal(envelope.Error, &plain) == nil && plain != "" {
				err = errors.New(plain)
			} else {
				err = fmt.Errorf("upstream error (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
			}
		}
	} else {
		msg := strings.TrimSpace(string(data))
		if msg == "" {
			msg = resp.Status
		}
		err = fmt.Errorf("upstream error (HTTP %d): %s", resp.StatusCode, msg)
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		return &TransientError{Status: resp.StatusCode, Err: err}
	}
	return err
}

// wire request types

type oaMsg struct {
	Role       string       `json:"role"`
	Content    string       `json:"content"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
	ToolCalls  []oaToolCall `json:"tool_calls,omitempty"`
}

type oaToolCall struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type"`
	Function oaFunc `json:"function"`
}

type oaFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type oaTool struct {
	Type     string   `json:"type"`
	Function oaToolFn `json:"function"`
}

type oaToolFn struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

type oaStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type oaReq struct {
	Model         string          `json:"model"`
	Messages      []oaMsg         `json:"messages"`
	Stream        bool            `json:"stream"`
	StreamOptions oaStreamOptions `json:"stream_options"`
	Tools         []oaTool        `json:"tools,omitempty"`
	MaxTokens     int             `json:"max_tokens,omitempty"`
	Temperature   *float64        `json:"temperature,omitempty"`
}

func oaRequest(req Request) oaReq {
	out := oaReq{
		Model:         req.Model,
		Stream:        true,
		StreamOptions: oaStreamOptions{IncludeUsage: true},
		MaxTokens:     req.MaxTokens,
		Temperature:   req.Temperature,
	}
	for _, m := range req.Messages {
		om := oaMsg{Role: string(m.Role), Content: m.Content, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			om.ToolCalls = append(om.ToolCalls, oaToolCall{
				ID:       tc.ID,
				Type:     "function",
				Function: oaFunc{Name: tc.Name, Arguments: string(tc.Args)},
			})
		}
		out.Messages = append(out.Messages, om)
	}
	for _, td := range req.Tools {
		out.Tools = append(out.Tools, oaTool{
			Type:     "function",
			Function: oaToolFn{Name: td.Name, Description: td.Description, Parameters: td.Parameters},
		})
	}
	return out
}

// wire chunk types

type oaChunkChoice struct {
	Index        int            `json:"index"`
	Delta        map[string]any `json:"delta"`
	FinishReason *string        `json:"finish_reason"`
}

type oaChunk struct {
	Choices []oaChunkChoice `json:"choices"`
	Usage   *map[string]any `json:"usage"`
}

type oaAccum struct {
	id   string
	name string
	args strings.Builder
}

func (o *OpenAI) oaReadSSE(ctx context.Context, body io.ReadCloser, ch chan Part) {
	defer close(ch)
	defer body.Close()
	send := func(p Part) {
		select {
		case ch <- p:
		case <-ctx.Done():
		}
	}

	tools := map[int]*oaAccum{}
	var tcOrder []int
	pendingFinish := ""
	var usage *Usage
	readErr := error(nil)
	finished := false
	finish := func() {
		if finished {
			return
		}
		finished = true
		for _, i := range tcOrder {
			a := tools[i]
			send(Part{Kind: "tool", Name: a.name, CallID: a.id, Args: json.RawMessage(a.args.String())})
		}
		switch {
		case pendingFinish != "":
			send(Part{Kind: "text", Finish: pendingFinish, Usage: usage})
		case readErr != nil:
			send(Part{Kind: "text", Finish: "error", Err: readErr})
		default:
			send(Part{Kind: "text", Finish: "error", Err: errors.New("stream ended without finish reason")})
		}
	}

	process := func(payload string) {
		if payload == "[DONE]" {
			finish()
			return
		}
		var chunk oaChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return // ignore malformed frame
		}
		if chunk.Usage != nil {
			usage = oaUsage(*chunk.Usage)
			return
		}
		for _, c := range chunk.Choices {
			delta := c.Delta
			if content, _ := delta["content"].(string); content != "" {
				send(Part{Kind: "text", Text: content})
			}
			rc, _ := delta["reasoning_content"].(string)
			if rc == "" {
				rc, _ = delta["reasoning"].(string)
			}
			if rc != "" {
				send(Part{Kind: "reasoning", Text: rc})
			}
			if tcs, _ := delta["tool_calls"].([]any); tcs != nil {
				for _, raw := range tcs {
					tc, ok := raw.(map[string]any)
					if !ok {
						continue
					}
					idx := int(oaNum(tc["index"]))
					a, ok := tools[idx]
					if !ok {
						a = &oaAccum{}
						tools[idx] = a
						tcOrder = append(tcOrder, idx)
					}
					if id, _ := tc["id"].(string); id != "" {
						a.id = id
					}
					fn, _ := tc["function"].(map[string]any)
					if fn == nil {
						continue
					}
					if name, _ := fn["name"].(string); name != "" {
						a.name = name
					}
					if args, _ := fn["arguments"].(string); args != "" {
						a.args.WriteString(args)
					}
				}
			}
		}
		for _, c := range chunk.Choices {
			if c.FinishReason != nil && *c.FinishReason != "" {
				pendingFinish = *c.FinishReason
			}
		}
	}

	rd := bufio.NewReader(body)
	var dataLines []string
	for {
		line, err := rd.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")
			switch {
			case line == "":
				if len(dataLines) > 0 {
					process(strings.Join(dataLines, "\n"))
					dataLines = nil
				}
			case strings.HasPrefix(line, "data:"):
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				readErr = err
			}
			break
		}
	}
	if len(dataLines) > 0 {
		process(strings.Join(dataLines, "\n"))
	}
	finish()
}

func oaNum(v any) float64 {
	f, _ := v.(float64)
	return f
}

func oaUsage(u map[string]any) *Usage {
	out := &Usage{
		Input:  int(oaNum(u["prompt_tokens"])),
		Output: int(oaNum(u["completion_tokens"])),
	}
	if d, _ := u["completion_tokens_details"].(map[string]any); d != nil {
		out.Reasoning = int(oaNum(d["reasoning_tokens"]))
	}
	if d, _ := u["prompt_tokens_details"].(map[string]any); d != nil {
		out.CacheRead = int(oaNum(d["cached_tokens"]))
	}
	return out
}
