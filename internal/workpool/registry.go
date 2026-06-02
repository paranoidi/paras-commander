package workpool

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/paranoidi/paras-commander/internal/pools"
)

// Registry maps pool names to limiters built from config definitions.
type Registry struct {
	pools map[string]*Pool
}

// NewRegistry builds an immutable registry from validated config pool definitions.
func NewRegistry(defs []pools.Def) *Registry {
	pools := make(map[string]*Pool, len(defs))
	for _, d := range defs {
		name := strings.TrimSpace(d.Name)
		if name == "" {
			continue
		}
		if _, exists := pools[name]; exists {
			continue
		}
		pools[name] = New(d.MaxParallel)
	}
	return &Registry{pools: pools}
}

// Pool returns the named pool, if defined.
func (r *Registry) Pool(name string) (*Pool, bool) {
	if r == nil {
		return nil, false
	}
	p, ok := r.pools[strings.TrimSpace(name)]
	return p, ok
}

// Names returns configured pool names sorted ascending.
func (r *Registry) Names() []string {
	if r == nil || len(r.pools) == 0 {
		return nil
	}
	out := make([]string, 0, len(r.pools))
	for name := range r.pools {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Acquire waits for a slot in the named pool and returns a release function.
// The caller must invoke release when work finishes (typically defer release()).
func (r *Registry) Acquire(ctx context.Context, name string) (release func(), err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("work pool name is empty")
	}
	p, ok := r.Pool(name)
	if !ok {
		return nil, fmt.Errorf("unknown work pool %q", name)
	}
	if err := p.Acquire(ctx); err != nil {
		return nil, err
	}
	return p.Release, nil
}
