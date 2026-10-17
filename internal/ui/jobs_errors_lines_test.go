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
