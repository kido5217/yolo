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
