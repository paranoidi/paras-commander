package keymap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultLeaderKeysUnique(t *testing.T) {
	if err := validateLeaderKeysPerScope(DefaultLeaderKeys()); err != nil {
		t.Fatal(err)
	}
}

func TestValidateLeaderKeysPerScopeAllowsCrossViewReuse(t *testing.T) {
	// Two different actions sharing the same letter in two independent view scopes is legal:
	// each rendered menu is a closed set, so the key only needs to be unique within its own menu.
	saved := leaderMenuViewSpecs
	defer func() { leaderMenuViewSpecs = saved }()
	leaderMenuViewSpecs = map[HelpViews]leaderMenuViewSpec{
		HelpCompare: {
			order:   []string{LeaderMenuGroupCompare},
			actions: map[string][]string{LeaderMenuGroupCompare: {ActionCompareClose}},
		},
		HelpDedup: {
			order:   []string{LeaderMenuGroupTree},
			actions: map[string][]string{LeaderMenuGroupTree: {ActionDedupCollapse}},
		},
	}
	keys := map[string]string{
		ActionCompareClose:  "c",
		ActionDedupCollapse: "c",
	}
	if err := validateLeaderKeysPerScope(keys); err != nil {
		t.Fatalf("cross-view reuse of %q should be legal: %v", "c", err)
	}
	if err := validateLeaderKeys(keys); err == nil {
		t.Fatal("naive validateLeaderKeys on the merged map should reject the duplicate")
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
	if keys[ActionJobsOpen] != "j" || keys[ActionMessagesOpen] != "l" || keys[ActionCommandsOpen] != "E" {
		t.Fatalf("views = jobs %q messages %q commands %q, want j / l / E", keys[ActionJobsOpen], keys[ActionMessagesOpen], keys[ActionCommandsOpen])
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
	if colFor(LeaderMenuGroupNavigation) != 2 || colFor(LeaderMenuGroupDisplay) != 3 || colFor(LeaderMenuGroupApp) != 3 {
		t.Fatalf("columns = nav %d display %d app %d, want 2 3 3", colFor(LeaderMenuGroupNavigation), colFor(LeaderMenuGroupDisplay), colFor(LeaderMenuGroupApp))
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
	if actions != 34 {
		t.Fatalf("action entries = %d, want 34", actions)
	}
}

func TestDefaultLeaderKeysAllowPunctuation(t *testing.T) {
	keys := DefaultLeaderKeys()
	if keys[ActionPanelToggleHidden] != "." || keys[ActionPanelMeta] != "," {
		t.Fatalf("view keys = hidden %q meta %q, want . / ,", keys[ActionPanelToggleHidden], keys[ActionPanelMeta])
	}
	if err := validateLeaderKeysPerScope(keys); err != nil {
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

// checkLeaderMenuEntriesForView asserts entries built for vm match leaderMenuViewSpecs[vm]'s
// group order, per-group action membership/order, and the spec's own column assignment.
func checkLeaderMenuEntriesForView(t *testing.T, vm HelpViews) {
	t.Helper()
	spec, ok := leaderMenuViewSpecs[vm]
	if !ok {
		t.Fatalf("no leaderMenuViewSpecs entry for %v", vm)
	}
	keys := DefaultLeaderKeys()
	entries := BuildLeaderMenuEntriesForView(keys, vm)
	if len(entries) == 0 {
		t.Fatal("expected non-empty entries")
	}

	var gotGroups []string
	groupActions := map[string][]string{}
	groupColumn := map[string]int{}
	var curGroup string
	for _, e := range entries {
		if e.GroupTitle != "" {
			curGroup = e.GroupTitle
			gotGroups = append(gotGroups, curGroup)
			groupColumn[curGroup] = e.GroupColumn
			continue
		}
		groupActions[curGroup] = append(groupActions[curGroup], e.ActionID)
		if e.GroupColumn != groupColumn[curGroup] {
			t.Fatalf("action %q GroupColumn = %d, want %d (matching its group header)", e.ActionID, e.GroupColumn, groupColumn[curGroup])
		}
	}

	var wantGroups []string
	for _, g := range spec.order {
		if len(spec.actions[g]) > 0 {
			wantGroups = append(wantGroups, g)
		}
	}
	if len(gotGroups) != len(wantGroups) {
		t.Fatalf("groups = %v, want %v", gotGroups, wantGroups)
	}
	for i := range wantGroups {
		if gotGroups[i] != wantGroups[i] {
			t.Fatalf("group[%d] = %q, want %q", i, gotGroups[i], wantGroups[i])
		}
		wantActions := spec.actions[wantGroups[i]]
		var wantBound []string
		for _, id := range wantActions {
			if _, ok := keys[id]; ok {
				wantBound = append(wantBound, id)
			}
		}
		gotActions := groupActions[wantGroups[i]]
		if len(gotActions) != len(wantBound) {
			t.Fatalf("group %q actions = %v, want %v", wantGroups[i], gotActions, wantBound)
		}
		for j := range wantBound {
			if gotActions[j] != wantBound[j] {
				t.Fatalf("group %q action[%d] = %q, want %q", wantGroups[i], j, gotActions[j], wantBound[j])
			}
		}
		wantCol := spec.column[wantGroups[i]]
		if groupColumn[wantGroups[i]] != wantCol {
			t.Fatalf("group %q column = %d, want %d", wantGroups[i], groupColumn[wantGroups[i]], wantCol)
		}
	}
}

func TestBuildLeaderMenuEntriesForViewCompare(t *testing.T) {
	checkLeaderMenuEntriesForView(t, HelpCompare)
}

func TestBuildLeaderMenuEntriesForViewDedup(t *testing.T) {
	checkLeaderMenuEntriesForView(t, HelpDedup)
}

func TestBuildLeaderMenuEntriesForViewJobs(t *testing.T) {
	checkLeaderMenuEntriesForView(t, HelpJobs)
}

func TestBuildLeaderMenuEntriesForViewCommands(t *testing.T) {
	checkLeaderMenuEntriesForView(t, HelpCommands)
}

func TestBuildLeaderMenuEntriesForViewMessages(t *testing.T) {
	checkLeaderMenuEntriesForView(t, HelpMessages)
}

func TestActionForLeaderKeyInViewScopedPerView(t *testing.T) {
	saved := leaderMenuViewSpecs
	defer func() { leaderMenuViewSpecs = saved }()
	leaderMenuViewSpecs = map[HelpViews]leaderMenuViewSpec{
		HelpCompare: {
			order:   []string{LeaderMenuGroupCompare},
			actions: map[string][]string{LeaderMenuGroupCompare: {ActionCompareClose}},
		},
		HelpDedup: {
			order:   []string{LeaderMenuGroupTree},
			actions: map[string][]string{LeaderMenuGroupTree: {ActionDedupCollapse}},
		},
	}
	b := &Bundle{LeaderKey: map[string]string{
		ActionCompareClose:  "c",
		ActionDedupCollapse: "c",
	}}

	id, ok := b.ActionForLeaderKeyInView('c', HelpCompare)
	if !ok || id != ActionCompareClose {
		t.Fatalf("Compare 'c' = %q %v, want %s", id, ok, ActionCompareClose)
	}
	id, ok = b.ActionForLeaderKeyInView('c', HelpDedup)
	if !ok || id != ActionDedupCollapse {
		t.Fatalf("Dedup 'c' = %q %v, want %s", id, ok, ActionDedupCollapse)
	}
	// A view with no leaderMenuViewSpecs entry (e.g. the browser) never resolves.
	if _, ok := b.ActionForLeaderKeyInView('c', HelpBrowser); ok {
		t.Fatal("HelpBrowser has no per-view leader menu; expected not found")
	}
	// A letter not bound in a given view's own scope is not found there, even though it's
	// bound in another view's scope.
	if _, ok := b.ActionForLeaderKeyInView('z', HelpCompare); ok {
		t.Fatal("unbound letter should not resolve")
	}
}

func TestActionForLeaderKeyInViewNilBundle(t *testing.T) {
	var b *Bundle
	if _, ok := b.ActionForLeaderKeyInView('c', HelpCompare); ok {
		t.Fatal("nil bundle should never resolve")
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
