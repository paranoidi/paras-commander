package host

import "testing"

type navRecorder struct{ dir, name string }

func (r *navRecorder) NavigatePanelToPath(_ int, path, selectName string) error {
	r.dir, r.name = path, selectName
	return nil
}

func TestNavigatePanelToEntryKeepsScheme(t *testing.T) {
	for _, tc := range []struct {
		path              string
		isDir             bool
		wantDir, wantName string
		wantReport        string
	}{
		{"fileindex:///lantern/meadow/chime.ogg", false, "fileindex:///lantern/meadow", "chime.ogg", "chime.ogg"},
		{"/lantern/meadow/chime.ogg", false, "/lantern/meadow", "chime.ogg", "chime.ogg"},
		{"fileindex:///lantern/meadow", true, "fileindex:///lantern/meadow", "", "meadow"},
	} {
		var r navRecorder
		report, err := NavigatePanelToEntry(&r, 0, tc.path, tc.isDir)
		if err != nil || r.dir != tc.wantDir || r.name != tc.wantName || report != tc.wantReport {
			t.Errorf("NavigatePanelToEntry(%q, %v) = %q,%v navigated %q/%q; want %q navigated %q/%q",
				tc.path, tc.isDir, report, err, r.dir, r.name, tc.wantReport, tc.wantDir, tc.wantName)
		}
	}
}
