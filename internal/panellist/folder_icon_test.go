package panellist

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestResolveFolderIconKindPriority(t *testing.T) {
	th := theme.Default()
	dir := localfs.Entry{Name: "alpha", Path: "/tmp/alpha", Type: localfs.EntryDirectory}

	ctx := FolderIconContext{OtherPanelPath: "/tmp/alpha", DiskUsageChrome: true}
	kind, ok := ResolveFolderIconKind(dir, ctx)
	if !ok || kind != theme.FolderIconOpen {
		t.Fatalf("open kind = %v ok=%v, want FolderIconOpen", kind, ok)
	}
	if th.FolderIcon(kind) != th.FolderIcon(theme.FolderIconOpen) {
		t.Fatalf("open icon mismatch")
	}

	ctx = FolderIconContext{OtherPanelPath: "/tmp/alpha", DiskPending: true, DiskUsageChrome: true}
	kind, ok = ResolveFolderIconKind(dir, ctx)
	if !ok || kind != theme.FolderIconScanning {
		t.Fatalf("scanning wins over open: kind = %v, want FolderIconScanning", kind)
	}

	ctx = FolderIconContext{DiskUsageChrome: true}
	kind, ok = ResolveFolderIconKind(dir, ctx)
	if !ok || kind != theme.FolderIconDefault {
		t.Fatalf("default kind = %v, want FolderIconDefault", kind)
	}
	if th.FolderIcon(kind) != th.FolderIcon(theme.FolderIconDefault) {
		t.Fatalf("default folder icon mismatch")
	}

	ctx = FolderIconContext{DiskExcluded: true, DiskUsageChrome: true}
	kind, ok = ResolveFolderIconKind(dir, ctx)
	if !ok || kind != theme.FolderIconExcluded {
		t.Fatalf("excluded kind = %v, want FolderIconExcluded", kind)
	}
	if th.FolderIcon(kind) != th.FolderIcon(theme.FolderIconExcluded) {
		t.Fatalf("excluded icon = %q, want %q", th.FolderIcon(kind), th.FolderIcon(theme.FolderIconExcluded))
	}
}

func TestResolveFolderIconKindTreeExpanded(t *testing.T) {
	th := theme.Default()
	dir := localfs.Entry{Name: "alpha", Path: "/tmp/alpha", Type: localfs.EntryDirectory}

	ctx := FolderIconContext{TreeExpanded: true}
	kind, ok := ResolveFolderIconKind(dir, ctx)
	if !ok || kind != theme.FolderIconTreeExpanded {
		t.Fatalf("tree-expanded kind = %v ok=%v, want FolderIconTreeExpanded", kind, ok)
	}
	if th.FolderIcon(kind) != th.FolderIcon(theme.FolderIconOpen) {
		t.Fatalf("tree-expanded icon mismatch with FolderIconOpen")
	}

	// Open-in-other-panel is the stronger signal and wins when both apply.
	ctx = FolderIconContext{OtherPanelPath: "/tmp/alpha", TreeExpanded: true}
	kind, ok = ResolveFolderIconKind(dir, ctx)
	if !ok || kind != theme.FolderIconOpen {
		t.Fatalf("open-in-other-panel should win over tree-expanded: kind = %v", kind)
	}
}

func TestResolveFolderIconKindNonDirectory(t *testing.T) {
	file := localfs.Entry{Name: "readme.txt", Path: "/tmp/readme.txt", Type: localfs.EntryFile}
	_, ok := ResolveFolderIconKind(file, FolderIconContext{})
	if ok {
		t.Fatal("non-directory should return false")
	}
}
