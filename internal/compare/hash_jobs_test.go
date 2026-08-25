package compare

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestHashJobsNeededSkipsUniqueSizePrimaryOnly(t *testing.T) {
	primary := []FileRecord{
		{Rel: "huge.mkv", Abs: pathloc.MustParse("/p/huge.mkv"), Size: 9_000_000_000},
		{Rel: "tiny.txt", Abs: pathloc.MustParse("/p/tiny.txt"), Size: 10},
	}
	secondary := []FileRecord{
		{Rel: "other.txt", Abs: pathloc.MustParse("/s/other.txt"), Size: 20},
	}
	jobs := hashJobsNeeded(primary, secondary)
	if len(jobs) != 0 {
		t.Fatalf("jobs = %+v, want none (no same-size peers)", jobs)
	}
}

func TestHashJobsNeededSkipsSamePathSizeMismatch(t *testing.T) {
	primary := []FileRecord{
		{Rel: "movie.mkv", Abs: pathloc.MustParse("/p/movie.mkv"), Size: 100},
	}
	secondary := []FileRecord{
		{Rel: "movie.mkv", Abs: pathloc.MustParse("/s/movie.mkv"), Size: 200},
	}
	jobs := hashJobsNeeded(primary, secondary)
	if len(jobs) != 0 {
		t.Fatalf("jobs = %+v, want none (same-path size mismatch)", jobs)
	}
}

func TestHashJobsNeededHashesSamePathSameSize(t *testing.T) {
	primary := []FileRecord{
		{Rel: "same.txt", Abs: pathloc.MustParse("/p/same.txt"), Size: 42},
	}
	secondary := []FileRecord{
		{Rel: "same.txt", Abs: pathloc.MustParse("/s/same.txt"), Size: 42},
	}
	jobs := hashJobsNeeded(primary, secondary)
	if len(jobs) != 2 {
		t.Fatalf("jobs = %d, want 2", len(jobs))
	}
	sides := map[int]bool{}
	for _, j := range jobs {
		if j.rel != "same.txt" {
			t.Fatalf("unexpected rel %q", j.rel)
		}
		sides[j.side] = true
	}
	if !sides[0] || !sides[1] {
		t.Fatalf("sides = %v, want both primary and secondary", sides)
	}
}

func TestHashJobsNeededHashesRelocatedCandidates(t *testing.T) {
	primary := []FileRecord{
		{Rel: "old/name.txt", Abs: pathloc.MustParse("/p/old/name.txt"), Size: 5},
	}
	secondary := []FileRecord{
		{Rel: "new/name.txt", Abs: pathloc.MustParse("/s/new/name.txt"), Size: 5},
	}
	jobs := hashJobsNeeded(primary, secondary)
	if len(jobs) != 2 {
		t.Fatalf("jobs = %d, want 2 (cross-path same size)", len(jobs))
	}
}

func TestHashJobsNeededEmptyWhenUnpairedSizesDiffer(t *testing.T) {
	primary := []FileRecord{
		{Rel: "only-p.bin", Abs: pathloc.MustParse("/p/only-p.bin"), Size: 111},
	}
	secondary := []FileRecord{
		{Rel: "only-s.bin", Abs: pathloc.MustParse("/s/only-s.bin"), Size: 222},
	}
	jobs := hashJobsNeeded(primary, secondary)
	if len(jobs) != 0 {
		t.Fatalf("jobs = %+v, want empty", jobs)
	}
}

func TestHashJobsNeededMixedSkipsAndKeeps(t *testing.T) {
	primary := []FileRecord{
		{Rel: "pair.txt", Abs: pathloc.MustParse("/p/pair.txt"), Size: 10},
		{Rel: "mismatch.txt", Abs: pathloc.MustParse("/p/mismatch.txt"), Size: 1},
		{Rel: "solo-big.mkv", Abs: pathloc.MustParse("/p/solo-big.mkv"), Size: 1_000_000},
		{Rel: "moved-a.txt", Abs: pathloc.MustParse("/p/moved-a.txt"), Size: 7},
	}
	secondary := []FileRecord{
		{Rel: "pair.txt", Abs: pathloc.MustParse("/s/pair.txt"), Size: 10},
		{Rel: "mismatch.txt", Abs: pathloc.MustParse("/s/mismatch.txt"), Size: 2},
		{Rel: "moved-b.txt", Abs: pathloc.MustParse("/s/moved-b.txt"), Size: 7},
	}
	jobs := hashJobsNeeded(primary, secondary)
	got := map[string]int{}
	for _, j := range jobs {
		got[j.rel]++
	}
	want := map[string]int{
		"pair.txt":    2,
		"moved-a.txt": 1,
		"moved-b.txt": 1,
	}
	if len(got) != len(want) {
		t.Fatalf("jobs rels = %v, want %v", got, want)
	}
	for rel, n := range want {
		if got[rel] != n {
			t.Fatalf("jobs[%q] = %d, want %d (full set %v)", rel, got[rel], n, got)
		}
	}
	if got["solo-big.mkv"] != 0 || got["mismatch.txt"] != 0 {
		t.Fatalf("unexpected jobs for skippable files: %v", got)
	}
}

func TestHashJobsNeededOrderedSmallestFirst(t *testing.T) {
	primary := []FileRecord{
		{Rel: "big.bin", Abs: pathloc.MustParse("/p/big.bin"), Size: 1000},
		{Rel: "tiny.txt", Abs: pathloc.MustParse("/p/tiny.txt"), Size: 3},
		{Rel: "mid.dat", Abs: pathloc.MustParse("/p/mid.dat"), Size: 50},
	}
	secondary := []FileRecord{
		{Rel: "big.bin", Abs: pathloc.MustParse("/s/big.bin"), Size: 1000},
		{Rel: "tiny.txt", Abs: pathloc.MustParse("/s/tiny.txt"), Size: 3},
		{Rel: "mid.dat", Abs: pathloc.MustParse("/s/mid.dat"), Size: 50},
	}
	jobs := hashJobsNeeded(primary, secondary)
	if len(jobs) != 6 {
		t.Fatalf("jobs = %d, want 6", len(jobs))
	}
	for i := 1; i < len(jobs); i++ {
		if jobs[i].size < jobs[i-1].size {
			t.Fatalf("jobs not ascending by size at %d: %+v then %+v", i, jobs[i-1], jobs[i])
		}
	}
	if jobs[0].size != 3 || jobs[len(jobs)-1].size != 1000 {
		t.Fatalf("size range = %d..%d, want 3..1000", jobs[0].size, jobs[len(jobs)-1].size)
	}
}
