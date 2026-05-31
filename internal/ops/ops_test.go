package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/testutil"
)

func TestResolveSourceNoSelectionUsesCursor(t *testing.T) {
	dir := t.TempDir()
	p := newTestPanel(t, dir, "a.txt", "b.txt")

	source, err := ResolveSource(&p)
	if err != nil {
		t.Fatalf("ResolveSource() error = %v", err)
	}
	if source.Kind != SourceCursor {
		t.Fatalf("source.Kind = %d, want SourceCursor", source.Kind)
	}
	if len(source.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(source.Entries))
	}
	if source.Entries[0].Name != "a.txt" {
		t.Fatalf("entry name = %q, want a.txt", source.Entries[0].Name)
	}
}

func TestResolveSourceWithSelectionUsesSelected(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.txt")
	bPath := filepath.Join(dir, "b.txt")
	testutil.WriteFile(t, aPath)
	testutil.WriteFile(t, bPath)

	p, err := panel.NewWithOptions(dir, localfs.DefaultListOptions(), nil)
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	p.SelectedPaths = map[string]bool{aPath: true, bPath: true}

	source, err := ResolveSource(&p)
	if err != nil {
		t.Fatalf("ResolveSource() error = %v", err)
	}
	if source.Kind != SourceSelected {
		t.Fatalf("source.Kind = %d, want SourceSelected", source.Kind)
	}
	if len(source.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(source.Entries))
	}
}

func TestResolveSourceSelectedIncludesPathNotInListing(t *testing.T) {
	dir := t.TempDir()
	otherDir := t.TempDir()
	aPath := filepath.Join(dir, "a.txt")
	remotePath := filepath.Join(otherDir, "remote.txt")
	testutil.WriteFile(t, aPath)
	testutil.WriteFile(t, remotePath)

	p, err := panel.NewWithOptions(dir, localfs.DefaultListOptions(), nil)
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	p.SelectedPaths = map[string]bool{aPath: true, remotePath: true}

	source, err := ResolveSource(&p)
	if err != nil {
		t.Fatalf("ResolveSource() error = %v", err)
	}
	if source.Kind != SourceSelected {
		t.Fatalf("source.Kind = %d, want SourceSelected", source.Kind)
	}
	if len(source.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(source.Entries))
	}
	seen := map[string]bool{}
	for _, e := range source.Entries {
		seen[e.Path] = true
	}
	if !seen[aPath] || !seen[remotePath] {
		t.Fatalf("missing paths: got %#v", seen)
	}
}

func TestResolvedSameAsSource(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	aaa := filepath.Join(dir, "aaa")
	if err := os.Mkdir(aaa, 0o755); err != nil {
		t.Fatal(err)
	}
	if !ResolvedSameAsSource(MustPath(aaa), MustPath(dir)) {
		t.Fatal("copy into parent dir with same basename should resolve to source path")
	}
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "other")
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if ResolvedSameAsSource(MustPath(other), MustPath(nested)) {
		t.Fatal("different destination directory should not resolve to source path")
	}
}

func TestSourcePathsPreservesEntryOrder(t *testing.T) {
	source := Source{
		Kind: SourceSelected,
		Entries: []localfs.Entry{
			{Name: "b.txt", Path: "/tmp/b.txt"},
			{Name: "a.txt", Path: "/tmp/a.txt"},
		},
	}

	paths := SourcePaths(source)

	want := []string{"/tmp/b.txt", "/tmp/a.txt"}
	if len(paths) != len(want) {
		t.Fatalf("len(paths) = %d, want %d", len(paths), len(want))
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("bookmarks[%d] = %q, want %q", i, paths[i], want[i])
		}
	}
}

func TestResolveSourceEmptyPanel(t *testing.T) {
	dir := t.TempDir()
	p, err := panel.New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = ResolveSource(&p)
	if err == nil {
		t.Fatal("ResolveSource() error = nil, want error")
	}
}

func TestResolveSourceSingleWithSelectionReturnsError(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.txt")
	bPath := filepath.Join(dir, "b.txt")
	testutil.WriteFile(t, aPath)
	testutil.WriteFile(t, bPath)

	p, err := panel.NewWithOptions(dir, localfs.DefaultListOptions(), nil)
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	p.SelectedPaths = map[string]bool{aPath: true}

	_, err = ResolveSourceSingle(&p)
	if err == nil {
		t.Fatal("ResolveSourceSingle() error = nil, want error for selected entries")
	}
}

func TestPlanRenameValid(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "old.txt")
	testutil.WriteFile(t, srcPath)

	entry := localfs.Entry{Name: "old.txt", Path: srcPath}
	plan, err := PlanRename(entry, "new.txt", dir)
	if err != nil {
		t.Fatalf("PlanRename() error = %v", err)
	}
	if plan.NewName != "new.txt" {
		t.Fatalf("NewName = %q, want new.txt", plan.NewName)
	}
	if plan.NewPath != filepath.Join(dir, "new.txt") {
		t.Fatalf("NewPath = %q, want %q", plan.NewPath, filepath.Join(dir, "new.txt"))
	}
}

func TestPlanRenameSameNameErrors(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "a.txt")
	testutil.WriteFile(t, srcPath)

	entry := localfs.Entry{Name: "a.txt", Path: srcPath}
	_, err := PlanRename(entry, "a.txt", dir)
	if err == nil {
		t.Fatal("PlanRename() error = nil, want error for same name")
	}
}

func TestPlanRenameEmptyName(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "a.txt")
	testutil.WriteFile(t, srcPath)

	entry := localfs.Entry{Name: "a.txt", Path: srcPath}
	_, err := PlanRename(entry, "", dir)
	if err == nil {
		t.Fatal("PlanRename() error = nil, want error for empty name")
	}
}

func TestPlanRenameWithPathSeparator(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "a.txt")
	testutil.WriteFile(t, srcPath)

	entry := localfs.Entry{Name: "a.txt", Path: srcPath}
	_, err := PlanRename(entry, "sub/new.txt", dir)
	if err == nil {
		t.Fatal("PlanRename() error = nil, want error for path separators")
	}
}

func TestExecuteRename(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "old.txt")
	testutil.WriteFile(t, srcPath)

	entry := localfs.Entry{Name: "old.txt", Path: srcPath}
	plan, err := PlanRename(entry, "new.txt", dir)
	if err != nil {
		t.Fatalf("PlanRename() error = %v", err)
	}

	if err := ExecuteRename(plan); err != nil {
		t.Fatalf("ExecuteRename() error = %v", err)
	}

	if _, err := os.Stat(plan.NewPath); err != nil {
		t.Fatalf("renamed file does not exist: %v", err)
	}
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Fatal("old file still exists")
	}
}

func TestPlanRenameTargetAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "a.txt")
	dstPath := filepath.Join(dir, "b.txt")
	testutil.WriteFile(t, srcPath)
	testutil.WriteFile(t, dstPath)

	entry := localfs.Entry{Name: "a.txt", Path: srcPath}
	_, err := PlanRename(entry, "b.txt", dir)
	if err == nil {
		t.Fatal("PlanRename() error = nil, want error for existing target")
	}
}

func TestPlanMkdir(t *testing.T) {
	dir := t.TempDir()

	plan, err := PlanMkdir("newdir", dir)
	if err != nil {
		t.Fatalf("PlanMkdir() error = %v", err)
	}
	if plan.Path != filepath.Join(dir, "newdir") {
		t.Fatalf("Path = %q, want %q", plan.Path, filepath.Join(dir, "newdir"))
	}
}

func TestPlanMkdirEmptyName(t *testing.T) {
	dir := t.TempDir()
	_, err := PlanMkdir("", dir)
	if err == nil {
		t.Fatal("PlanMkdir() error = nil, want error for empty name")
	}
}

func TestPlanMkdirAbsolute(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "sub", "newdir")

	plan, err := PlanMkdir(abs, dir)
	if err != nil {
		t.Fatalf("PlanMkdir() error = %v", err)
	}
	if plan.Path != abs {
		t.Fatalf("Path = %q, want %q", plan.Path, abs)
	}
}

func TestExecuteMkdir(t *testing.T) {
	dir := t.TempDir()

	plan, err := PlanMkdir("newdir", dir)
	if err != nil {
		t.Fatalf("PlanMkdir() error = %v", err)
	}

	if err := ExecuteMkdir(plan); err != nil {
		t.Fatalf("ExecuteMkdir() error = %v", err)
	}

	info, err := os.Stat(plan.Path)
	if err != nil {
		t.Fatalf("stat after mkdir: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("created path is not a directory")
	}
}

func TestPlanMkdirExisting(t *testing.T) {
	dir := t.TempDir()
	existingDir := filepath.Join(dir, "existing")
	if err := os.Mkdir(existingDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	_, err := PlanMkdir("existing", dir)
	if err == nil {
		t.Fatal("PlanMkdir() error = nil, want error for existing dir")
	}
}

func TestPlanDelete(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a.txt")
	testutil.WriteFile(t, filePath)

	entry := localfs.Entry{Name: "a.txt", Path: filePath}
	source := Source{Kind: SourceCursor, Entries: []localfs.Entry{entry}}

	plan, err := PlanDelete(source, true, "permanent")
	if err != nil {
		t.Fatalf("PlanDelete() error = %v", err)
	}
	if len(plan.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(plan.Entries))
	}
	if !plan.ConfirmFirst {
		t.Fatal("ConfirmFirst = false, want true")
	}
}

func TestExecuteDeleteFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a.txt")
	testutil.WriteFile(t, filePath)

	entry := localfs.Entry{Name: "a.txt", Path: filePath}
	plan := DeletePlan{
		Entries:      []localfs.Entry{entry},
		DeleteMode:   "permanent",
		ConfirmFirst: false,
	}

	if err := ExecuteDelete(plan); err != nil {
		t.Fatalf("ExecuteDelete() error = %v", err)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatal("file still exists after delete")
	}
}

func TestExecuteDeleteDirectory(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	testutil.WriteFile(t, filepath.Join(subDir, "inside.txt"))

	entry := localfs.Entry{Name: "subdir", Path: subDir, Type: localfs.EntryDirectory}
	plan := DeletePlan{
		Entries:      []localfs.Entry{entry},
		DeleteMode:   "permanent",
		ConfirmFirst: false,
	}

	if err := ExecuteDelete(plan); err != nil {
		t.Fatalf("ExecuteDelete() error = %v", err)
	}
	if _, err := os.Stat(subDir); !os.IsNotExist(err) {
		t.Fatal("directory still exists after delete")
	}
}

func TestExecuteDeleteDirectoryWithHiddenFile(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "nested")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	testutil.WriteFile(t, filepath.Join(subDir, ".hidden"))
	testutil.WriteFile(t, filepath.Join(subDir, "visible.txt"))

	entry := localfs.Entry{Name: "nested", Path: subDir, Type: localfs.EntryDirectory}
	plan := DeletePlan{
		Entries:      []localfs.Entry{entry},
		DeleteMode:   "permanent",
		ConfirmFirst: false,
	}
	if err := ExecuteDelete(plan); err != nil {
		t.Fatalf("ExecuteDelete() error = %v", err)
	}
	if _, err := os.Stat(subDir); !os.IsNotExist(err) {
		t.Fatal("directory still exists after delete with hidden child")
	}
}

func TestOpsErrorNestedDeleteMessage(t *testing.T) {
	t.Parallel()
	inner := &os.PathError{Op: "remove", Path: "/tmp/x", Err: errors.New("directory not empty")}
	wrapped := fmt.Errorf(`remove "/tmp/x": %w`, inner)
	opErr := &Error{Op: "delete", Text: "failed to delete leaf", Err: wrapped}
	got := opErr.Error()
	want := "delete: failed to delete leaf (directory not empty)"
	if got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestPlanDeleteEmpty(t *testing.T) {
	_, err := PlanDelete(Source{}, true, "permanent")
	if err == nil {
		t.Fatal("PlanDelete() error = nil, want error for empty source")
	}
}

func TestCountDirectories(t *testing.T) {
	entries := []localfs.Entry{
		{Name: "a.txt", Type: localfs.EntryFile},
		{Name: "sub", Type: localfs.EntryDirectory},
		{Name: "b.txt", Type: localfs.EntryFile},
	}
	if n := CountDirectories(entries); n != 1 {
		t.Fatalf("CountDirectories = %d, want 1", n)
	}
}

func TestParseNumericMode(t *testing.T) {
	tests := []struct {
		input string
		want  os.FileMode
	}{
		{"755", 0o755},
		{"644", 0o644},
		{"0644", 0o644},
		{"777", 0o777},
		{"0o755", 0o755},
		{"0O644", 0o644},
	}
	for _, tt := range tests {
		mode, err := parseNumericMode(tt.input)
		if err != nil {
			t.Fatalf("parseNumericMode(%q) error = %v", tt.input, err)
		}
		if mode != tt.want {
			t.Fatalf("parseNumericMode(%q) = %o, want %o", tt.input, mode, tt.want)
		}
	}
}

func TestParseNumericModeInvalid(t *testing.T) {
	invalid := []string{"", "abc", "99", "8", "12345"}
	for _, input := range invalid {
		_, err := parseNumericMode(input)
		if err == nil {
			t.Fatalf("parseNumericMode(%q) error = nil, want error", input)
		}
	}
}

func TestParseSymbolicMode(t *testing.T) {
	tests := []struct {
		input string
		want  os.FileMode
	}{
		{"u+r", 0o400},
		{"g+w", 0o020},
		{"o+x", 0o001},
		{"a+rx", 0o555},
		{"u=rw,go=r", 0o644},
		{"a+w", 0o222},
		{"u=rwx,g=rx,o=", 0o750},
	}
	for _, tt := range tests {
		mode, err := parseSymbolicMode(tt.input)
		if err != nil {
			t.Fatalf("parseSymbolicMode(%q) error = %v", tt.input, err)
		}
		if mode != tt.want {
			t.Fatalf("parseSymbolicMode(%q) = %o, want %o", tt.input, mode, tt.want)
		}
	}
}

func TestParseSymbolicModeSubtract(t *testing.T) {
	// Starting from 0, subtract write = 0.
	mode, err := parseSymbolicMode("a-w")
	if err != nil {
		t.Fatalf("parseSymbolicMode(a-w) error = %v", err)
	}
	if mode != 0 {
		t.Fatalf("parseSymbolicMode(a-w) = %o, want 0 (no bits set, subtract from 0)", mode)
	}

	// Set bits first, then subtract.
	mode, err = parseSymbolicMode("a=rwx,a-w")
	if err != nil {
		t.Fatalf("parseSymbolicMode(a=rwx,a-w) error = %v", err)
	}
	if mode != 0o555 {
		t.Fatalf("parseSymbolicMode(a=rwx,a-w) = %o, want 555", mode)
	}
}

func TestParseMode(t *testing.T) {
	tests := []struct {
		input string
		want  os.FileMode
	}{
		{"755", 0o755},
		{"u+r", 0o400},
		{"u=rwx,g=rx,o=", 0o750},
	}
	for _, tt := range tests {
		mode, err := parseMode(tt.input)
		if err != nil {
			t.Fatalf("parseMode(%q) error = %v", tt.input, err)
		}
		if mode != tt.want {
			t.Fatalf("parseMode(%q) = %o, want %o", tt.input, mode, tt.want)
		}
	}
}

func TestPlanChmod(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a.txt")
	testutil.WriteFile(t, filePath)

	entry := localfs.Entry{Name: "a.txt", Path: filePath}
	source := Source{Kind: SourceCursor, Entries: []localfs.Entry{entry}}

	plan, err := PlanChmod(source, "755")
	if err != nil {
		t.Fatalf("PlanChmod() error = %v", err)
	}
	if plan.Mode != 0o755 {
		t.Fatalf("Mode = %o, want 755", plan.Mode)
	}
}

func TestPlanChmodEmptyMode(t *testing.T) {
	entry := localfs.Entry{Name: "a.txt", Path: "/tmp/a.txt"}
	source := Source{Kind: SourceCursor, Entries: []localfs.Entry{entry}}
	_, err := PlanChmod(source, "")
	if err == nil {
		t.Fatal("PlanChmod() error = nil, want error for empty mode")
	}
}

func TestPlanChownValid(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a.txt")
	testutil.WriteFile(t, filePath)

	entry := localfs.Entry{Name: "a.txt", Path: filePath}
	source := Source{Kind: SourceCursor, Entries: []localfs.Entry{entry}}

	// Use current user/group (should work without privileges when staying same? actually chown requires privileges)
	// Just test planning, not execution.
	plan, err := PlanChown(source, "", "")
	if err != nil {
		t.Fatalf("PlanChown() error = %v", err)
	}
	if plan.UID != -1 || plan.GID != -1 {
		t.Fatalf("blank fields should give -1, got uid=%d gid=%d", plan.UID, plan.GID)
	}
}

func TestPlanChownBlankField(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a.txt")
	testutil.WriteFile(t, filePath)

	entry := localfs.Entry{Name: "a.txt", Path: filePath}
	source := Source{Kind: SourceCursor, Entries: []localfs.Entry{entry}}

	plan, err := PlanChown(source, "", "users")
	if err != nil {
		t.Fatalf("PlanChown() blank user error = %v", err)
	}
	if plan.UID != -1 {
		t.Fatalf("blank user should give uid=-1, got %d", plan.UID)
	}
}

func TestPlanSymlink(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.txt")
	testutil.WriteFile(t, targetPath)
	linkDir := filepath.Join(t.TempDir(), "link.txt")

	plan, err := PlanSymlink(targetPath, linkDir, dir, filepath.Dir(linkDir))
	if err != nil {
		t.Fatalf("PlanSymlink() error = %v", err)
	}
	if plan.Target != targetPath {
		t.Fatalf("Target = %q, want %q", plan.Target, targetPath)
	}
	if plan.LinkPath != linkDir {
		t.Fatalf("LinkPath = %q, want %q", plan.LinkPath, linkDir)
	}
}

func TestPlanSymlinkTargetNotExist(t *testing.T) {
	dir := t.TempDir()
	_, err := PlanSymlink("/nonexistent", "/tmp/link", dir, "/tmp")
	if err == nil {
		t.Fatal("PlanSymlink() error = nil, want error for nonexistent target")
	}
}

func TestPlanSymlinkEmptyFields(t *testing.T) {
	dir := t.TempDir()
	_, err := PlanSymlink("", "link", dir, dir)
	if err == nil {
		t.Fatal("PlanSymlink() error = nil for empty target")
	}
	_, err = PlanSymlink("target", "", dir, dir)
	if err == nil {
		t.Fatal("PlanSymlink() error = nil for empty link path")
	}
}

func TestExecuteSymlink(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.txt")
	testutil.WriteFile(t, targetPath)
	linkPath := filepath.Join(dir, "mylink")

	plan, err := PlanSymlink(targetPath, linkPath, dir, dir)
	if err != nil {
		t.Fatalf("PlanSymlink() error = %v", err)
	}

	if err := ExecuteSymlink(plan); err != nil {
		t.Fatalf("ExecuteSymlink() error = %v", err)
	}

	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("lstat after symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("created path is not a symlink")
	}
}

func TestPlanHardlink(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "source.txt")
	testutil.WriteFile(t, srcPath)
	destPath := filepath.Join(dir, "hardlink.txt")

	plan, err := PlanHardlink(srcPath, destPath, dir, dir)
	if err != nil {
		t.Fatalf("PlanHardlink() error = %v", err)
	}
	if plan.Source != srcPath {
		t.Fatalf("Source = %q, want %q", plan.Source, srcPath)
	}
	if plan.NewPath != destPath {
		t.Fatalf("NewPath = %q, want %q", plan.NewPath, destPath)
	}
}

func TestPlanHardlinkToDirectory(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	_, err := PlanHardlink(subDir, filepath.Join(dir, "link"), dir, dir)
	if err == nil {
		t.Fatal("PlanHardlink() error = nil, want error for directory")
	}
}

func TestPlanHardlinkSamePath(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a.txt")
	testutil.WriteFile(t, filePath)

	_, err := PlanHardlink(filePath, filePath, dir, dir)
	if err == nil {
		t.Fatal("PlanHardlink() error = nil, want error for same path")
	}
}

func TestPlanHardlinkNonexistentSource(t *testing.T) {
	dir := t.TempDir()
	_, err := PlanHardlink("/nonexistent", filepath.Join(dir, "link"), dir, dir)
	if err == nil {
		t.Fatal("PlanHardlink() error = nil, want error")
	}
}

// --- Copy/Move tests from plan 03 ---

func TestCopyRegularFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "source.txt")
	if err := os.WriteFile(srcFile, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	opts := Options{PreservePermissions: true, PreserveTimestamps: true, CopyBufferKiB: 4}
	var progressCalls int
	progress := func(_, _ string, done int, bytes int64) {
		progressCalls++
	}

	done, bytes, err := ExecuteCopy(context.Background(), MustPaths(srcFile), MustPath(dstDir), opts, ProgressEmitThrottle{}, progress, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteCopy error = %v", err)
	}
	if done != 1 {
		t.Fatalf("done files = %d, want 1", done)
	}
	if bytes <= 0 {
		t.Fatalf("done bytes = %d, want > 0", bytes)
	}
	if progressCalls != 1 {
		t.Fatalf("progress calls = %d, want 1", progressCalls)
	}

	dstFile := filepath.Join(dstDir, "source.txt")
	data, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("content = %q, want %q", string(data), "hello world")
	}
}

func TestExecuteCopyUsingPlanMatchesExecuteCopy(t *testing.T) {
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "same.txt")
	if err := os.WriteFile(srcFile, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	dstA := filepath.Join(t.TempDir(), "da")
	dstB := filepath.Join(t.TempDir(), "db")
	if err := os.Mkdir(dstA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dstB, 0o755); err != nil {
		t.Fatal(err)
	}
	opts := Options{PreservePermissions: false, PreserveTimestamps: false, CopyBufferKiB: 8, CowFileCloning: false}

	plan, _, _, err := BuildCopyPlanWithTotals(MustPaths(srcFile), MustPath(dstA))
	if err != nil {
		t.Fatalf("BuildCopyPlanWithTotals: %v", err)
	}
	da, ba, err := ExecuteCopyUsingPlan(context.Background(), plan, MustPaths(srcFile), MustPath(dstA), opts, ProgressEmitThrottle{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteCopyUsingPlan: %v", err)
	}
	db, bb, err := ExecuteCopy(context.Background(), MustPaths(srcFile), MustPath(dstB), opts, ProgressEmitThrottle{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteCopy: %v", err)
	}
	if da != db || ba != bb {
		t.Fatalf("usingPlan vs ExecuteCopy: files %d vs %d bytes %d vs %d", da, db, ba, bb)
	}
	gotA, _ := os.ReadFile(filepath.Join(dstA, "same.txt"))
	gotB, _ := os.ReadFile(filepath.Join(dstB, "same.txt"))
	if string(gotA) != string(gotB) {
		t.Fatalf("dest content differs")
	}
}

func TestExecuteCopyUsingPlanNilPlanErrors(t *testing.T) {
	_, _, err := ExecuteCopyUsingPlan(context.Background(), nil, nil, pathloc.Path{}, DefaultOptions(), ProgressEmitThrottle{}, nil, nil, nil)
	if err == nil {
		t.Fatal("ExecuteCopyUsingPlan(nil plan) error = nil, want error")
	}
}

func TestCopyGranularProgressMultipleEmits(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "big.bin")
	payload := make([]byte, 16*1024)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	if err := os.WriteFile(srcFile, payload, 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	opts := Options{PreservePermissions: false, PreserveTimestamps: false, CopyBufferKiB: 4}
	throttle := ProgressEmitThrottle{MinBytes: 1, MinInterval: time.Nanosecond}
	var progressCalls int
	var lastBytes int64 = -1
	progress := func(_, _ string, _ int, bytes int64) {
		if bytes < lastBytes {
			t.Fatalf("DoneBytes went backwards: %d -> %d", lastBytes, bytes)
		}
		lastBytes = bytes
		progressCalls++
	}
	done, gotBytes, err := ExecuteCopy(context.Background(), MustPaths(srcFile), MustPath(dstDir), opts, throttle, progress, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteCopy: %v", err)
	}
	if done != 1 || gotBytes != int64(len(payload)) {
		t.Fatalf("done=%d bytes=%d want files=1 bytes=%d", done, gotBytes, len(payload))
	}
	if progressCalls < 3 {
		t.Fatalf("progressCalls = %d, want >= 3", progressCalls)
	}
}

func TestExecuteCopyCanceledBeforeFirstItem(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "a.txt")
	if err := os.WriteFile(srcFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opts := Options{CopyBufferKiB: 4}
	_, _, err := ExecuteCopy(ctx, MustPaths(srcFile), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, nil, nil)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestCopyDirectoryRecursively(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create src structure
	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("root"), 0o644); err != nil {
		t.Fatalf("write root file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "child.txt"), []byte("child"), 0o644); err != nil {
		t.Fatalf("write child file: %v", err)
	}

	opts := Options{PreservePermissions: false, PreserveTimestamps: false, CopyBufferKiB: 4}
	done, _, err := ExecuteCopy(context.Background(), MustPaths(srcDir), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteCopy error = %v", err)
	}
	// srcDir itself + file.txt + sub dir + child.txt = 4 items
	if done != 4 {
		t.Fatalf("done files = %d, want 4", done)
	}

	// Check structure.
	base := filepath.Base(srcDir)
	if _, err := os.Stat(filepath.Join(dstDir, base, "file.txt")); err != nil {
		t.Fatalf("missing root file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, base, "sub", "child.txt")); err != nil {
		t.Fatalf("missing child file: %v", err)
	}
}

func TestCopySymlinkAsSymlink(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	target := filepath.Join(srcDir, "target.txt")
	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	linkPath := filepath.Join(srcDir, "link.lnk")
	if err := os.Symlink("target.txt", linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	opts := Options{CopyBufferKiB: 4}
	done, _, err := ExecuteCopy(context.Background(), MustPaths(linkPath), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteCopy error = %v", err)
	}
	if done != 1 {
		t.Fatalf("done files = %d, want 1", done)
	}

	// Verify symlink was recreated (not followed).
	dstLink := filepath.Join(dstDir, "link.lnk")
	linkTarget, err := os.Readlink(dstLink)
	if err != nil {
		t.Fatalf("read destination symlink: %v", err)
	}
	if linkTarget != "target.txt" {
		t.Fatalf("symlink target = %q, want %q", linkTarget, "target.txt")
	}
}

func TestMoveRenameFastPath(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "move.txt")
	if err := os.WriteFile(srcFile, []byte("move me"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	opts := Options{CopyBufferKiB: 4}
	done, _, err := ExecuteMove(context.Background(), MustPaths(srcFile), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteMove error = %v", err)
	}
	if done < 1 {
		t.Fatalf("done files = %d, want >=1", done)
	}

	// Source should no longer exist.
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Fatalf("source still exists, err = %v", err)
	}

	// Destination should exist.
	dstFile := filepath.Join(dstDir, "move.txt")
	if _, err := os.Stat(dstFile); err != nil {
		t.Fatalf("destination missing: %v", err)
	}
}

func TestMoveMultiSourceRenameFastPath(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	a := filepath.Join(srcDir, "a.txt")
	b := filepath.Join(srcDir, "b.txt")
	if err := os.WriteFile(a, []byte("a"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(b, []byte("b"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	opts := Options{CopyBufferKiB: 4}
	done, _, err := ExecuteMove(context.Background(), MustPaths(a, b), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteMove: %v", err)
	}
	if done != 2 {
		t.Fatalf("done files = %d, want 2", done)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if _, err := os.Stat(filepath.Join(dstDir, name)); err != nil {
			t.Fatalf("missing dst %s: %v", name, err)
		}
	}
	if _, err := os.Stat(a); !os.IsNotExist(err) {
		t.Fatalf("source a still exists: %v", err)
	}
	if _, err := os.Stat(b); !os.IsNotExist(err) {
		t.Fatalf("source b still exists: %v", err)
	}
}

func TestMoveRenameProgressUsesLogicalPaths(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "x.txt")
	if err := os.WriteFile(srcFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	wantDst := filepath.Join(dstDir, "x.txt")
	var gotSrc, gotDst string
	progress := func(srcPath, dstPath string, doneFiles int, doneBytes int64) {
		_ = doneFiles
		_ = doneBytes
		gotSrc, gotDst = srcPath, dstPath
	}
	opts := Options{CopyBufferKiB: 4}
	if _, _, err := ExecuteMove(context.Background(), MustPaths(srcFile), MustPath(dstDir), opts, ProgressEmitThrottle{}, progress, nil, nil); err != nil {
		t.Fatalf("ExecuteMove: %v", err)
	}
	if gotSrc != srcFile {
		t.Fatalf("progress source = %q, want %q", gotSrc, srcFile)
	}
	if gotDst != wantDst {
		t.Fatalf("progress dest = %q, want %q", gotDst, wantDst)
	}
}

func TestMoveOverwriteExistingDestViaRename(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "file.txt")
	if err := os.WriteFile(srcFile, []byte("new"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	dstFile := filepath.Join(dstDir, "file.txt")
	if err := os.WriteFile(dstFile, []byte("old"), 0o644); err != nil {
		t.Fatalf("write dest: %v", err)
	}

	resolved := false
	resolver := func(src, dst string, facts FileConflictFacts) (bool, error) {
		_ = src
		_ = dst
		_ = facts
		resolved = true
		return true, nil
	}

	opts := Options{CopyBufferKiB: 4}
	done, doneBytes, err := ExecuteMove(context.Background(), MustPaths(srcFile), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, resolver, nil)
	if err != nil {
		t.Fatalf("ExecuteMove error = %v", err)
	}
	if !resolved {
		t.Fatal("conflict resolver was not called")
	}
	if done != 1 {
		t.Fatalf("done files = %d, want 1", done)
	}
	if doneBytes != 0 {
		t.Fatalf("done bytes = %d, want 0 for rename fast path", doneBytes)
	}
	data, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("content = %q, want %q", string(data), "new")
	}
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Fatalf("source should be gone after move, err = %v", err)
	}
}

func TestMoveExistingDestNoResolver(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "file.txt")
	if err := os.WriteFile(srcFile, []byte("new"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	dstFile := filepath.Join(dstDir, "file.txt")
	if err := os.WriteFile(dstFile, []byte("old"), 0o644); err != nil {
		t.Fatalf("write dest: %v", err)
	}

	opts := Options{CopyBufferKiB: 4}
	_, _, err := ExecuteMove(context.Background(), MustPaths(srcFile), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, nil, nil)
	if err == nil {
		t.Fatal("ExecuteMove error = nil, want conflict resolver error")
	}
	if _, err := os.Stat(srcFile); err != nil {
		t.Fatalf("source should remain: %v", err)
	}
	data, _ := os.ReadFile(dstFile)
	if string(data) != "old" {
		t.Fatalf("dest content = %q, want %q", string(data), "old")
	}
}

func TestMoveSkipExistingDestViaRename(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "file.txt")
	if err := os.WriteFile(srcFile, []byte("new"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	dstFile := filepath.Join(dstDir, "file.txt")
	if err := os.WriteFile(dstFile, []byte("old"), 0o644); err != nil {
		t.Fatalf("write dest: %v", err)
	}

	resolved := false
	resolver := func(src, dst string, facts FileConflictFacts) (bool, error) {
		_ = src
		_ = dst
		_ = facts
		resolved = true
		return false, nil
	}

	opts := Options{CopyBufferKiB: 4}
	done, doneBytes, err := ExecuteMove(context.Background(), MustPaths(srcFile), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, resolver, nil)
	if err != nil {
		t.Fatalf("ExecuteMove error = %v", err)
	}
	if !resolved {
		t.Fatal("conflict resolver was not called")
	}
	if done != 0 {
		t.Fatalf("done files = %d, want 0 on skip", done)
	}
	if doneBytes != 0 {
		t.Fatalf("done bytes = %d, want 0 on skip", doneBytes)
	}
	if _, err := os.Stat(srcFile); err != nil {
		t.Fatalf("source should remain after skip: %v", err)
	}
	data, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(data) != "old" {
		t.Fatalf("dest content = %q, want %q", string(data), "old")
	}
}

func TestMoveConflictResolverCancel(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	a := filepath.Join(srcDir, "a.txt")
	b := filepath.Join(srcDir, "b.txt")
	if err := os.WriteFile(a, []byte("a"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(b, []byte("b"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}
	dstB := filepath.Join(dstDir, "b.txt")
	if err := os.WriteFile(dstB, []byte("old-b"), 0o644); err != nil {
		t.Fatalf("write dest b: %v", err)
	}

	calls := 0
	resolver := func(src, dst string, facts FileConflictFacts) (bool, error) {
		_ = src
		_ = dst
		_ = facts
		calls++
		return false, fmt.Errorf("canceled by user")
	}

	opts := Options{CopyBufferKiB: 4}
	_, _, err := ExecuteMove(context.Background(), MustPaths(a, b), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, resolver, nil)
	if err == nil {
		t.Fatal("ExecuteMove error = nil, want cancel error")
	}
	if calls != 1 {
		t.Fatalf("resolver calls = %d, want 1", calls)
	}
	if _, err := os.Stat(a); err != nil {
		t.Fatalf("source a should remain after rollback: %v", err)
	}
	if _, err := os.Stat(b); err != nil {
		t.Fatalf("source b should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "a.txt")); err == nil {
		t.Fatal("partial rename of a should have been rolled back")
	}
	data, _ := os.ReadFile(dstB)
	if string(data) != "old-b" {
		t.Fatalf("dest b content = %q, want %q", string(data), "old-b")
	}
}

func TestResolveDestination(t *testing.T) {
	dir := t.TempDir()

	// Destination is a directory.
	dst := ResolveDestination(MustPath("/src/file.txt"), MustPath(dir))
	if dst.String() != filepath.Join(dir, "file.txt") {
		t.Fatalf("dir dest = %q, want %q", dst, filepath.Join(dir, "file.txt"))
	}

	// Destination is a full path (non-existent).
	dst = ResolveDestination(MustPath("/src/file.txt"), MustPath("/some/path/newfile.txt"))
	if dst.String() != "/some/path/newfile.txt" {
		t.Fatalf("file dest = %q, want /some/path/newfile.txt", dst)
	}
}

func TestDestinationIsDirAtEnqueue(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !DestinationIsDirAtEnqueue(MustPath(dir)) {
		t.Fatal("existing directory should report true")
	}
	if DestinationIsDirAtEnqueue(MustPath(file)) {
		t.Fatal("regular file should report false")
	}
	missing := filepath.Join(dir, "nope")
	if DestinationIsDirAtEnqueue(MustPath(missing)) {
		t.Fatal("missing path should report false")
	}
}

func TestConflictResolverOverwrite(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "file.txt")
	if err := os.WriteFile(srcFile, []byte("new"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	dstFile := filepath.Join(dstDir, "file.txt")
	if err := os.WriteFile(dstFile, []byte("old"), 0o644); err != nil {
		t.Fatalf("write dest: %v", err)
	}

	resolved := false
	resolver := func(src, dst string, facts FileConflictFacts) (bool, error) {
		_ = facts
		resolved = true
		return true, nil // overwrite
	}

	opts := Options{CopyBufferKiB: 4}
	done, doneBytes, err := ExecuteCopy(context.Background(), MustPaths(srcFile), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, resolver, nil)
	if err != nil {
		t.Fatalf("ExecuteCopy error = %v", err)
	}
	if !resolved {
		t.Fatal("conflict resolver was not called")
	}
	if done != 1 {
		t.Fatalf("done files = %d, want 1", done)
	}
	if doneBytes <= 0 {
		t.Fatalf("done bytes = %d, want > 0", doneBytes)
	}

	data, _ := os.ReadFile(dstFile)
	if string(data) != "new" {
		t.Fatalf("content = %q, want %q", string(data), "new")
	}
}

func TestConflictResolverSkip(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "file.txt")
	if err := os.WriteFile(srcFile, []byte("new"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	dstFile := filepath.Join(dstDir, "file.txt")
	if err := os.WriteFile(dstFile, []byte("old"), 0o644); err != nil {
		t.Fatalf("write dest: %v", err)
	}

	resolved := false
	resolver := func(src, dst string, facts FileConflictFacts) (bool, error) {
		_ = facts
		resolved = true
		return false, nil // skip
	}

	opts := Options{CopyBufferKiB: 4}
	done, doneBytes, err := ExecuteCopy(context.Background(), MustPaths(srcFile), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, resolver, nil)
	if err != nil {
		t.Fatalf("ExecuteCopy error = %v", err)
	}
	if !resolved {
		t.Fatal("conflict resolver was not called")
	}
	if done != 0 {
		t.Fatalf("done files = %d, want 0 when skipping conflict destination", done)
	}
	if doneBytes != 0 {
		t.Fatalf("done bytes = %d, want 0 when skipping", doneBytes)
	}

	// Content should still be "old".
	data, _ := os.ReadFile(dstFile)
	if string(data) != "old" {
		t.Fatalf("content = %q, want %q", string(data), "old")
	}
}

func TestCopyRejectsSpecialFiles(t *testing.T) {
	// This test can only run on Linux where we can create a FIFO.
	if _, err := os.Stat("/dev/null"); err != nil {
		t.Skip("/dev/null not available, skipping special file test")
	}

	dstDir := t.TempDir()

	opts := Options{CopyBufferKiB: 4}
	_, _, err := ExecuteCopy(context.Background(), MustPaths("/dev/null"), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for special file, got nil")
	}
}

// --- Test helpers ---

func newTestPanel(t *testing.T, dir string, filenames ...string) panel.State {
	t.Helper()
	for _, name := range filenames {
		testutil.WriteFile(t, filepath.Join(dir, name))
	}
	p, err := panel.New(dir)
	if err != nil {
		t.Fatalf("panel.New(%q): %v", dir, err)
	}
	return p
}
