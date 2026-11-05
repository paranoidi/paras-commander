package fschange

import (
	"context"
	"testing"
)

type recordSink struct{ got [][]Change }

func (r *recordSink) Apply(b []Change) { r.got = append(r.got, b) }

func TestMultiFansOutInOrderAndSkipsNil(t *testing.T) {
	a, b := &recordSink{}, &recordSink{}
	if Multi() != nil || Multi(nil, nil) != nil {
		t.Fatal("no sinks must give a nil Sink")
	}
	if got := Multi(nil, a); got != Sink(a) {
		t.Fatalf("one sink must come back as itself, got %T", got)
	}
	ctx := WithSink(context.Background(), Multi(a, nil, b))
	Report(ctx, Change{Kind: Removed, Path: "/orchard//harbor.txt"})
	Report(ctx, Change{Kind: Created, Path: "/orchard/willow.txt"})
	for name, s := range map[string]*recordSink{"a": a, "b": b} {
		if len(s.got) != 2 || s.got[0][0].Path != "/orchard/harbor.txt" || s.got[1][0].Kind != Created {
			t.Errorf("sink %s got %+v, want both batches in order with cleaned paths", name, s.got)
		}
	}
}
