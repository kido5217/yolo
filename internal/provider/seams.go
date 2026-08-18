package provider

import (
	"context"
	"fmt"
	"net/http"
)

// NewWithSeams builds an offline registry whose catalog comes from seam
// (one synthetic model per provider id) instead of the live kido/zen
// catalogs. The engine tests use it so unit tests never hit the network.
func NewWithSeams(ctx context.Context, dataDir string, seam func(providerID string) (Info, Model, error)) (*Registry, error) {
	return &Registry{client: http.DefaultClient, defProvider: "kido", defModel: "q", seam: seam}, nil
}

// NewStaticForTest builds a fully offline registry: kido/q (default; no key)
// plus opencode/gpt-5-nano (key-required placeholder). Server tests use it so
// no test ever touches the network.
func NewStaticForTest() *Registry {
	return &Registry{
		client:      http.DefaultClient,
		defProvider: "kido",
		defModel:    "q",
		info: []Info{
			{
				ID: "kido", Name: "Kido", Source: "builtin",
				BaseURL: "https://ai.kido.ws/v1",
				KeyRequired: false, KeyLoaded: true,
				Models: []Model{{
					ID: "q", Name: "Qwen", Family: "qwen", Adapter: "openai",
					ToolCall: true, Reasoning: true,
					Context: 100000, Output: 16384,
				}},
			},
			{
				ID: "opencode", Name: "OpenCode Zen", Source: "builtin",
				BaseURL: "https://opencode.ai/zen/v1",
				KeyRequired: true, Env: []string{"OPENCODE_API_KEY"},
				Models: []Model{{
					ID: "gpt-5-nano", Name: "GPT-5 Nano", Family: "openai", Adapter: "openai",
					ToolCall: true, Reasoning: true,
					Context: 400000, Output: 16384,
				}},
			},
		},
	}
}

// resolveSeam resolves ref through the test seam, if set.
func (r *Registry) resolveSeam(pid, mid string) (Info, Model, bool, error) {
	if r.seam == nil {
		return Info{}, Model{}, false, nil
	}
	i, m, err := r.seam(pid)
	if err != nil {
		return Info{}, Model{}, true, err
	}
	if m.ID != mid {
		return i, Model{}, true, fmt.Errorf("unknown model %q in provider %s", mid, pid)
	}
	return i, m, true, nil
}
