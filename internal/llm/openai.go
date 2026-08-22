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
	httpReq, err := http.NewRequestWithContext(
		ctx, "POST", strings.TrimRight(req.BaseURL, "/")+"/chat/completions", bytes.NewReader(body),
	)
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
		return PartStream{}, upstreamError(resp)
	}
	ch := make(chan Part, 64)
	go o.oaReadSSE(ctx, resp.Body, ch)
	return PartStream{Parts: ch}, nil
}

// upstreamError builds an error from a non-2xx response (body ≤ 4KB drained;
// 429/5xx wrapped in *TransientError); the message prefers {"error":{...}}.
func upstreamError(resp *http.Response) error {
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
				err = fmt.Errorf("upstream error (http %d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
			}
		}
	} else {
		msg := strings.TrimSpace(string(data))
		if msg == "" {
			msg = resp.Status
		}
		err = fmt.Errorf("upstream error (http %d): %s", resp.StatusCode, msg)
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
			Function: oaToolFn(td),
		})
	}
	return out
}

// wire chunk types

type oaDeltaFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type oaDeltaToolCall struct {
	Index    float64     `json:"index"`
	ID       string      `json:"id"`
	Function oaDeltaFunc `json:"function"`
}

// oaDelta is the typed stream delta: only the fields the driver reads
// (content, reasoning, tool_calls) exist, so a chunk decodes without per-
// token map allocation and interface boxing.
type oaDelta struct {
	Content          string            `json:"content"`
	ReasoningContent string            `json:"reasoning_content"`
	Reasoning        string            `json:"reasoning"`
	ToolCalls        []oaDeltaToolCall `json:"tool_calls"`
}

type oaChunkChoice struct {
	Index        int     `json:"index"`
	Delta        oaDelta `json:"delta"`
	FinishReason *string `json:"finish_reason"`
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

	process := func(payload []byte) {
		if bytes.Equal(payload, sseDone) {
			finish()
			return
		}
		var chunk oaChunk
		if err := json.Unmarshal(payload, &chunk); err != nil {
			return // ignore malformed frame
		}
		if chunk.Usage != nil {
			usage = oaUsage(*chunk.Usage)
			return
		}
		for _, c := range chunk.Choices {
			d := c.Delta
			if d.Content != "" {
				send(Part{Kind: "text", Text: d.Content})
			}
			rc := d.ReasoningContent
			if rc == "" {
				rc = d.Reasoning
			}
			if rc != "" {
				send(Part{Kind: "reasoning", Text: rc})
			}
			for _, tc := range d.ToolCalls {
				idx := int(tc.Index)
				a, ok := tools[idx]
				if !ok {
					a = &oaAccum{}
					tools[idx] = a
					tcOrder = append(tcOrder, idx)
				}
				if tc.ID != "" {
					a.id = tc.ID
				}
				if tc.Function.Name != "" {
					a.name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					a.args.WriteString(tc.Function.Arguments)
				}
			}
		}
		for _, c := range chunk.Choices {
			if c.FinishReason != nil && *c.FinishReason != "" {
				pendingFinish = *c.FinishReason
			}
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
		// [DONE] already ended the stream: stop reading so a body that
		// never terminates does not hold the engine round hostage
		// (anReadSSE parity).
		if finished {
			break
		}
	}
	if len(dataVals) > 0 && !finished {
		process(joinDataLines(dataVals))
	}
	finish()
}

// sseDataPrefix is the SSE "data:" field marker; sseDone is the OpenAI
// stream terminator.
var (
	sseDataPrefix = []byte("data:")
	sseDone       = []byte("[DONE]")
)

// joinDataLines joins the trimmed "data:" values of one SSE frame with
// '\n' (multi-line data fields are valid SSE).
func joinDataLines(vals [][]byte) []byte {
	n := len(vals) - 1
	for _, v := range vals {
		n += len(v)
	}
	out := make([]byte, 0, n)
	for i, v := range vals {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, v...)
	}
	return out
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
