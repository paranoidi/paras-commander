package ops

import "testing"

func TestSummarizePlanCountsDirsAndBytes(t *testing.T) {
	plan := []PlanItem{
		{IsDir: true},
		{IsDir: false, IsSymlink: true},
		{IsDir: false, FileSize: 100},
		{IsDir: false, FileSize: 50},
	}
	items, dirs, bytes := SummarizePlan(plan)
	if items != 4 {
		t.Fatalf("items = %d want 4", items)
	}
	if dirs != 1 {
		t.Fatalf("dirs = %d want 1", dirs)
	}
	if bytes != 150 {
		t.Fatalf("bytes = %d want 150", bytes)
	}
}
