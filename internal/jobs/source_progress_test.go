package jobs

import (
	"slices"
	"testing"
)

// TestDeferSourceStaysQueued: a deferred source is queued again, starting another source does
// not finish it, and every phase change moves SourceVersion.
func TestDeferSourceStaysQueued(t *testing.T) {
	t.Parallel()
	j := &Job{sourcePhases: newSourceProgress(3)}
	versions := []uint32{j.SourceVersion()}
	step := func(f func(int32), i int32) {
		f(i)
		versions = append(versions, j.SourceVersion())
	}
	step(j.StartSource, 0)
	step(j.DeferSource, 0)
	step(j.StartSource, 1)
	step(j.StartSource, 2)
	if got, want := j.SourcePhases(), []SourcePhase{SourceQueued, SourceFinished, SourceStarted}; !slices.Equal(got, want) {
		t.Fatalf("phases = %v, want %v", got, want)
	}
	step(j.StartSource, 0)
	if got, want := j.SourcePhases(), []SourcePhase{SourceStarted, SourceFinished, SourceFinished}; !slices.Equal(got, want) {
		t.Fatalf("phases after restarting the deferred source = %v, want %v", got, want)
	}
	if j.ActiveSource() != 0 {
		t.Fatalf("ActiveSource = %d, want 0", j.ActiveSource())
	}
	for i := 1; i < len(versions); i++ {
		if versions[i] == versions[i-1] {
			t.Fatalf("SourceVersion unchanged by step %d: %v", i, versions)
		}
	}
	j.DeferSource(1)
	j.DeferSource(1)
	if v := j.SourceVersion(); v != versions[len(versions)-1]+1 {
		t.Fatalf("deferring an already queued source moved the version: %d", v)
	}
}
