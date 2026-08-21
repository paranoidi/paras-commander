package keymap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultLeaderKeysUnique(t *testing.T) {
	if err := validateLeaderKeys(DefaultLeaderKeys()); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultLeaderKeysAllowCasePairs(t *testing.T) {
	keys := DefaultLeaderKeys()
	if keys[ActionMove] != "m" || keys[ActionFileMkdir] != "M" {
		t.Fatalf("move/mkdir = %q / %q, want m / M", keys[ActionMove], keys[ActionFileMkdir])
	}
	if keys[ActionAppShowHelp] != "?" {
		t.Fatalf("help = %q, want ?", keys[ActionAppShowHelp])
	}
	if keys[ActionJobsOpen] != "J" || keys[ActionMessagesOpen] != "L" || keys[ActionCommandsOpen] != "E" {
		t.Fatalf("views = jobs %q messages %q commands %q, want J / L / E", keys[ActionJobsOpen], keys[ActionMessagesOpen], keys[ActionCommandsOpen])
	}
}

func TestBuildLeaderMenuEntriesGroupedOrder(t *testing.T) {
	keys := map[string]string{
		ActionFileDelete:  "d",
		ActionAppShowHelp: "?",
	}
	entries := BuildLeaderMenuEntries(keys)
	if len(entries) != 4 {
		t.Fatalf("len = %d, want 4 (File+delete, App+help)", len(entries))
	}
	if entries[0].GroupTitle != LeaderMenuGroupFile || entries[0].GroupColumn != 0 || entries[1].ActionID != ActionFileDelete {
		t.Fatalf("file group = %+v, %+v", entries[0], entries[1])
	}
	if entries[2].GroupTitle != LeaderMenuGroupApp || entries[2].GroupColumn != 3 || entries[3].ActionID != ActionAppShowHelp {
		t.Fatalf("app group = %+v, %+v", entries[2], entries[3])
	}
}

func TestBuildLeaderMenuEntriesFileOrder(t *testing.T) {
	keys := DefaultLeaderKeys()
	entries := BuildLeaderMenuEntries(keys)
	var fileActions []string
	inFile := false
	for _, e := range entries {
		if e.GroupTitle == LeaderMenuGroupFile {
			inFile = true
			continue
		}
		if inFile {
			if e.GroupTitle != "" {
				break
			}
			fileActions = append(fileActions, e.ActionID)
		}
	}
	wantAll := leaderMenuGroupActions[LeaderMenuGroupFile]
	var want []string
	for _, actionID := range wantAll {
		if _, ok := keys[actionID]; ok {
			want = append(want, actionID)
		}
	}
	if len(fileActions) != len(want) {
		t.Fatalf("file actions = %d, want %d", len(fileActions), len(want))
	}
	for i := range want {
		if fileActions[i] != want[i] {
			t.Fatalf("file[%d] = %q, want %q", i, fileActions[i], want[i])
		}
	}
}

func TestBuildLeaderMenuEntriesGroupColumns(t *testing.T) {
	keys := DefaultLeaderKeys()
	entries := BuildLeaderMenuEntries(keys)
	colFor := func(group string) int {
		for _, e := range entries {
			if e.GroupTitle == group {
				return e.GroupColumn
			}
		}
		return -1
	}
	if colFor(LeaderMenuGroupFile) != 0 || colFor(LeaderMenuGroupSelection) != 1 || colFor(LeaderMenuGroupView) != 1 || colFor(LeaderMenuGroupTools) != 2 {
		t.Fatalf("columns = file %d selection %d view %d tools %d, want 0 1 1 2", colFor(LeaderMenuGroupFile), colFor(LeaderMenuGroupSelection), colFor(LeaderMenuGroupView), colFor(LeaderMenuGroupTools))
	}
	if colFor(LeaderMenuGroupNavigation) != 3 || colFor(LeaderMenuGroupDisplay) != 3 || colFor(LeaderMenuGroupApp) != 3 {
		t.Fatalf("columns = nav %d display %d app %d, want 3 3 3", colFor(LeaderMenuGroupNavigation), colFor(LeaderMenuGroupDisplay), colFor(LeaderMenuGroupApp))
	}
}

func TestLeaderMenuKeysOverrideAndOmit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keybindings.toml")
	body := `[leader_key]
"file.mkdir" = "k"
"file.delete" = ""
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, modeUser, _, _, err := parseKeybindingsFile([]byte(body), path)
	if err != nil {
		t.Fatal(err)
	}
	merged := mergeLeaderKeys(DefaultLeaderKeys(), modeUser)
	if merged[ActionFileMkdir] != "k" {
		t.Fatalf("mkdir key = %q, want k", merged[ActionFileMkdir])
	}
	if _, ok := merged[ActionFileDelete]; ok {
		t.Fatalf("delete should be omitted, still in map: %v", merged[ActionFileDelete])
	}
}

func TestLeaderMenuKeysCasePairAllowed(t *testing.T) {
	keys := map[string]string{
		ActionMove:      "m",
		ActionFileMkdir: "M",
	}
	if err := validateLeaderKeys(keys); err != nil {
		t.Fatalf("m/M pair should be valid: %v", err)
	}
}

func TestLeaderMenuKeysExactDuplicateRejected(t *testing.T) {
	keys := map[string]string{
		ActionPanelFindDialog:     "f",
		ActionPanelFindDuplicates: "f",
	}
	if err := validateLeaderKeys(keys); err == nil {
		t.Fatal("expected conflict for duplicate f")
	}
}

func TestDefaultBundleLeaderKey(t *testing.T) {
	b, err := DefaultBundle()
	if err != nil {
		t.Fatal(err)
	}
	if len(b.LeaderKey) == 0 {
		t.Fatal("LeaderKey should have defaults")
	}
	entries := b.LeaderMenuEntries()
	if len(entries) == 0 {
		t.Fatal("expected leader menu entries")
	}
	actions := 0
	for _, e := range entries {
		if e.GroupTitle != "" {
			continue
		}
		actions++
		if e.ActionID == "" || e.Label == "" || e.Key == 0 {
			t.Fatalf("invalid entry: %+v", e)
		}
	}
	if actions != 33 {
		t.Fatalf("action entries = %d, want 33", actions)
	}
}

func TestDefaultLeaderKeysAllowPunctuation(t *testing.T) {
	keys := DefaultLeaderKeys()
	if keys[ActionPanelToggleHidden] != "." || keys[ActionPanelMeta] != "," {
		t.Fatalf("view keys = hidden %q meta %q, want . / ,", keys[ActionPanelToggleHidden], keys[ActionPanelMeta])
	}
	if err := validateLeaderKeys(keys); err != nil {
		t.Fatal(err)
	}
}

func TestBuildLeaderMenuEntriesViewGroup(t *testing.T) {
	keys := DefaultLeaderKeys()
	entries := BuildLeaderMenuEntries(keys)
	var viewActions []string
	inView := false
	for _, e := range entries {
		if e.GroupTitle == LeaderMenuGroupView {
			inView = true
			continue
		}
		if inView {
			if e.GroupTitle != "" {
				break
			}
			viewActions = append(viewActions, e.ActionID)
		}
	}
	want := leaderMenuGroupActions[LeaderMenuGroupView]
	if len(viewActions) != len(want) {
		t.Fatalf("view actions = %v, want %v", viewActions, want)
	}
	for i := range want {
		if viewActions[i] != want[i] {
			t.Fatalf("view[%d] = %q, want %q", i, viewActions[i], want[i])
		}
	}
}

func TestParseLeaderKeyRejectsNonString(t *testing.T) {
	body := `[leader_key]
"file.mkdir" = ["m"]
`
	_, _, _, _, _, err := parseKeybindingsFile([]byte(body), "test.toml")
	if err == nil || !strings.Contains(err.Error(), "expected string") {
		t.Fatalf("err = %v, want expected string", err)
	}
}
