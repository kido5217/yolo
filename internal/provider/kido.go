package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// kidoFallback is the static model list used whenever the live probe fails,
// times out, or parses to nothing. Startup must never block/fail on network.
var kidoFallback = []Model{{
	ID: "Qwen3.8-27B", Name: "Qwen3.8-27B", Adapter: "openai",
	ToolCall: true, Reasoning: true, Context: 262144, Output: 32768,
}}

// FetchKido probes {baseURL}/models (llamacpp/vLLM shape) and maps live
// models; on noNet, network failure, bad status, or parse failure it returns
// the static fallback. It does not return an error for network problems.
func FetchKido(ctx context.Context, baseURL string, timeoutMS int, noNet bool) ([]Model, error) {
	if noNet {
		return kidoFallback, nil
	}
	httpc := &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond}
	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		return kidoFallback, nil
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return kidoFallback, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return kidoFallback, nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return kidoFallback, nil
	}
	var parsed struct {
		Data []struct {
			ID   string `json:"id"`
			Meta struct {
				NCtx int `json:"n_ctx"`
			} `json:"meta"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil || len(parsed.Data) == 0 {
		return kidoFallback, nil
	}
	out := make([]Model, 0, len(parsed.Data))
	for _, d := range parsed.Data {
		out = append(out, Model{
			ID: d.ID, Name: d.ID, Adapter: "openai",
			ToolCall: true, Reasoning: true,
			Context: d.Meta.NCtx, Output: min(32768, d.Meta.NCtx/8),
		})
	}
	return out, nil
}
