package ui

import (
	"reflect"
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestJobErrorLinesFromJobState(t *testing.T) {
	if got := JobErrorLines(JobEntry{Status: "running"}); len(got) != 0 {
		t.Fatalf("lines = %v, want none", got)
	}
	src := pathloc.PathsForTest("/orchard/thistle")[0]
	e := JobEntry{
		Status: "failed",
		Error:  "1 item(s) failed\nsecond line",
		FailedItems: []jobs.FailedItem{
			{Item: ops.PlanItem{Src: src}, Error: "permission denied"},
		},
	}
	want := []string{"/orchard/thistle: permission denied", "1 item(s) failed"}
	if got := JobErrorLines(e); !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %v, want %v", got, want)
	}
}

func TestJobsPaneOrder(t *testing.T) {
	got := JobsPaneOrder(true, true, true)
	want := []JobsPane{JobsPaneList, JobsPaneConflict, JobsPaneDetail, JobsPaneErrors, JobsPaneSucceeded}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if got := JobsPaneOrder(false, false, true); !reflect.DeepEqual(got, []JobsPane{JobsPaneList, JobsPaneDetail, JobsPaneSucceeded}) {
		t.Fatalf("order = %v", got)
	}
}

func TestWrapJobsPaneLinesWrapsLongLine(t *testing.T) {
	rect := Rect{X: 0, Y: 0, Width: 24, Height: 10}
	long := "harbor lantern meadow thistle orchard willow pebble"
	got := WrapJobsPaneLines([]string{long, "short"}, rect)
	if len(got) < 3 {
		t.Fatalf("lines = %v, want long line wrapped into several rows", got)
	}
	for _, l := range got {
		if len([]rune(l)) > rect.Width-4 {
			t.Fatalf("line %q wider than %d", l, rect.Width-4)
		}
	}
}

func TestJobsMaximizedRectCoversBothColumns(t *testing.T) {
	l := Layout{Primary: Rect{X: 0, Y: 1, Width: 40, Height: 20}, Secondary: Rect{X: 40, Y: 1, Width: 39, Height: 20}}
	want := Rect{X: 0, Y: 1, Width: 79, Height: 20}
	if got := JobsMaximizedRect(l); got != want {
		t.Fatalf("rect = %+v, want %+v", got, want)
	}
}
