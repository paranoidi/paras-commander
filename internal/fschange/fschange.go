// Package fschange carries reports of local filesystem changes from the operations that make them
// to the subscribers that keep derived state (the disk-usage cache, meta columns, the file index)
// in step. It travels on a
// context, like ops.WithElevator, so a call path without a sink reports nothing.
package fschange

import (
	"context"
	"path/filepath"
)

// Kind is what happened to a path.
type Kind uint8

const (
	// Renamed: From now lives at Path (a replaced Path is gone).
	Renamed Kind = iota
	// Removed: Path and everything under it is gone.
	Removed
	// Created: Path is new, or a file at Path was overwritten.
	Created
	// Dirty: Path changed in a way whose sizes are unknown (an archive tool wrote it, a delete
	// stopped part way).
	Dirty
)

// Change is one reported effect on a clean local host path.
type Change struct {
	Kind Kind
	Path string
	// From is the old path of a Renamed change.
	From string
	// Bytes is a file's apparent size (Created, and Renamed files); 0 for directories.
	Bytes int64
	// IsDir says whether Path is a directory, so a subscriber can skip subtree work for files.
	IsDir bool
}

// Sink receives batches of changes. The changes of one batch happened together (a mass rename's
// swap); batches arrive in the order they happened. Apply must be cheap: it is called from job
// workers and from the UI goroutine.
type Sink interface {
	Apply(batch []Change)
}

type sinkKey struct{}

// WithSink returns ctx carrying s; a nil s returns ctx unchanged.
func WithSink(ctx context.Context, s Sink) context.Context {
	if s == nil {
		return ctx
	}
	return context.WithValue(ctx, sinkKey{}, s)
}

// WithoutSink returns ctx with no sink, for work in scratch directories that must not be reported.
func WithoutSink(ctx context.Context) context.Context {
	return context.WithValue(ctx, sinkKey{}, Sink(nil))
}

// Report hands batch, with every path cleaned, to ctx's sink. A context without a sink, or an
// empty batch, reports nothing.
func Report(ctx context.Context, batch ...Change) {
	s := sinkFrom(ctx)
	if len(batch) == 0 || s == nil {
		return
	}
	for i := range batch {
		batch[i].Path = filepath.Clean(batch[i].Path)
		if batch[i].From != "" {
			batch[i].From = filepath.Clean(batch[i].From)
		}
	}
	s.Apply(batch)
}

// Enabled reports whether ctx carries a sink, so a caller can skip the Lstat a report needs.
func Enabled(ctx context.Context) bool {
	return sinkFrom(ctx) != nil
}

func sinkFrom(ctx context.Context) Sink {
	if ctx == nil {
		return nil
	}
	s, _ := ctx.Value(sinkKey{}).(Sink)
	return s
}

// multi fans a batch out to several sinks.
type multi []Sink

func (m multi) Apply(batch []Change) {
	for _, s := range m {
		s.Apply(batch)
	}
}

// Multi returns a sink handing every batch to each non-nil sink in order, or nil when there is
// none. Sinks share the batch and must not modify it. A typed nil pointer in an interface is not
// detected here: a sink's Apply must accept its nil receiver (diskusage.Engine and
// fileindex.Index do).
func Multi(sinks ...Sink) Sink {
	var out multi
	for _, s := range sinks {
		if s != nil {
			out = append(out, s)
		}
	}
	switch len(out) {
	case 0:
		return nil
	case 1:
		return out[0]
	}
	return out
}
