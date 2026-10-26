package jobs

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// estimateHost is jobsHostStub with an active panel and a disk-usage cache.
type estimateHost struct {
	jobsHostStub
	active  *panel.State
	painter ui.DiskUsagePainter
}

func (h estimateHost) ActivePanel() *panel.State             { return h.active }
func (h estimateHost) DiskUsagePainter() ui.DiskUsagePainter { return h.painter }

type sizeOnlyPainter map[string]int64

func (p sizeOnlyPainter) ByteSize(path string) (int64, bool) { n, ok := p[path]; return n, ok }
func (sizeOnlyPainter) FileCount(string) (int64, bool)       { return 0, false }
func (sizeOnlyPainter) PendingForPanel(string, int) bool     { return false }
func (sizeOnlyPainter) DiskScanBusy() bool                   { return false }
func (sizeOnlyPainter) DiskScanExcluded(string, bool, uint64, bool, func(string) bool) bool {
	return false
}
func (sizeOnlyPainter) IsKnownExcluded(string) bool { return false }

func TestAddTransferJobSeedsSizeEstimate(t *testing.T) {
	t.Parallel()
	const (
		ledger  = "/orchard/ledger.csv"
		archive = "/orchard/archive"
		pending = "/orchard/pending"
	)
	active := &panel.State{Path: pathloc.MustParse("/orchard")}
	active.SetOffListingMeta(map[string]localfs.Entry{
		ledger:  {Path: ledger, Type: localfs.EntryFile, Size: 500},
		archive: {Path: archive, Type: localfs.EntryDirectory},
		pending: {Path: pending, Type: localfs.EntryDirectory},
	})
	host := estimateHost{active: active, painter: sizeOnlyPainter{archive: 9500}}

	tests := []struct {
		name      string
		typ       jobs.Type
		sources   []string
		wantBytes int64
		wantHints []int64
	}{
		{"copy with every size known", jobs.TypeCopy, []string{ledger, archive}, 10000, nil},
		{"copy takes a partial estimate", jobs.TypeCopy, []string{ledger, pending}, 500, nil},
		{"move with every size known", jobs.TypeMove, []string{ledger, archive}, 10000, []int64{500, 9500}},
		{"move with an unknown source has no estimate", jobs.TypeMove, []string{ledger, pending}, 0, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			state := jobs.NewState()
			h := New(Deps{Host: host, Model: &ui.Model{}, State: state, Config: config.Default()})
			h.AddTransferJob(TransferJobRequest{Type: tc.typ, Sources: tc.sources, Dest: "/harbor"})
			all := state.AllJobs()
			if len(all) != 1 {
				t.Fatalf("jobs = %d, want 1", len(all))
			}
			j := all[0]
			if j.EstimatedBytes != tc.wantBytes {
				t.Fatalf("EstimatedBytes = %d, want %d", j.EstimatedBytes, tc.wantBytes)
			}
			if tc.typ == jobs.TypeMove && j.TotalBytes != tc.wantBytes {
				t.Fatalf("TotalBytes = %d, want %d", j.TotalBytes, tc.wantBytes)
			}
			if len(j.SourceSizeHints) != len(tc.wantHints) {
				t.Fatalf("SourceSizeHints = %v, want %v", j.SourceSizeHints, tc.wantHints)
			}
			for i := range tc.wantHints {
				if j.SourceSizeHints[i] != tc.wantHints[i] {
					t.Fatalf("SourceSizeHints = %v, want %v", j.SourceSizeHints, tc.wantHints)
				}
			}
		})
	}
}
