package metacmds

import (
	"context"
	"slices"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// Provider is a Go column provider: a meta column a built-in feature computes in-process, listed
// in the meta picker beside the meta.toml entries and shown, cached, sorted and filtered like them.
type Provider struct {
	Name        string
	Description string
	Column      string // panel header; empty means use Name
	Order       int    // left-to-right sort key (lower = closer to name)
	// Section is the meta picker heading the provider is listed under, below the meta.toml
	// entries; the picker shows the provider by its column title there.
	Section string
	// Values returns the column values of entries, all directly in dir, keyed by entry path; an
	// entry left out shows an empty cell. It is called once per listing (and once more per batch
	// of entries a refresh adds) on a worker goroutine, never on the UI thread, and should stop
	// early when ctx is canceled.
	Values func(ctx context.Context, dir string, entries []localfs.Entry) (map[string]string, error)
	// Available, when set, reports whether the provider is offered at all (e.g. its database
	// opens). It is called with each meta file load, which may be on the UI thread.
	Available func() bool
}

// entry is the provider as a meta entry (MetaEntry.Provider set).
func (p *Provider) entry() MetaEntry {
	col := p.Column
	if col == "" {
		col = p.Name
	}
	return MetaEntry{Name: p.Name, Description: p.Description, Column: col, Order: p.Order, Provider: p}
}

// WithProviders returns mf's entries followed by one entry per available provider; a provider replaces a
// meta.toml entry of the same name. mf is not modified. A nil mf gives the providers alone, and
// no providers gives mf itself.
func WithProviders(mf *MetaFile, providers []*Provider) *MetaFile {
	providers = slices.DeleteFunc(slices.Clone(providers), func(p *Provider) bool {
		return p.Available != nil && !p.Available()
	})
	if len(providers) == 0 {
		return mf
	}
	out := &MetaFile{}
	taken := make(map[string]bool, len(providers))
	for _, p := range providers {
		taken[p.Name] = true
	}
	if mf != nil {
		out.ShellPatterns = mf.ShellPatterns
		for _, e := range mf.Entries {
			if !taken[e.Name] {
				out.Entries = append(out.Entries, e)
			}
		}
	}
	for _, p := range providers {
		out.Entries = append(out.Entries, p.entry())
	}
	return out
}
