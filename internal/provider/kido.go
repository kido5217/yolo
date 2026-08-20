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
// the static fallback. Network problems never block or fail startup, so the
// result carries no error. client may be nil (http.DefaultClient); the probe
// is bounded by timeoutMS either way. Entries with an empty id or non-positive
// n_ctx are skipped; if every entry is skipped the static fallback is
// returned.
func FetchKido(ctx context.Context, baseURL string, timeoutMS int, noNet bool, client *http.Client) []Model {
	if noNet {
		return kidoFallback
	}
	if client == nil {
		client = http.DefaultClient
	}
	fctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(fctx, "GET", strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		return kidoFallback
	}
	resp, err := client.Do(req)
	if err != nil {
		return kidoFallback
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return kidoFallback
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return kidoFallback
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
		return kidoFallback
	}
	out := make([]Model, 0, len(parsed.Data))
	for _, d := range parsed.Data {
		if d.ID == "" || d.Meta.NCtx <= 0 {
			continue
		}
		out = append(out, Model{
			ID: d.ID, Name: d.ID, Adapter: "openai",
			ToolCall: true, Reasoning: true,
			Context: d.Meta.NCtx, Output: min(32768, d.Meta.NCtx/8),
		})
	}
	if len(out) == 0 {
		return kidoFallback
	}
	return out
}
