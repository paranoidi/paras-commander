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
	if th.FolderIconGlyph(kind) != th.FolderIconGlyph(theme.FolderIconOpen) {
		t.Fatalf("open glyph mismatch")
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
	if th.FolderIconGlyph(kind) != th.FolderIconGlyph(theme.FolderIconDefault) {
		t.Fatalf("default folder glyph mismatch")
	}

	ctx = FolderIconContext{DiskExcluded: true, DiskUsageChrome: true}
	kind, ok = ResolveFolderIconKind(dir, ctx)
	if !ok || kind != theme.FolderIconExcluded {
		t.Fatalf("excluded kind = %v, want FolderIconExcluded", kind)
	}
	if th.FolderIconGlyph(kind) != th.FolderIconGlyph(theme.FolderIconExcluded) {
		t.Fatalf("excluded glyph = %q, want %q", th.FolderIconGlyph(kind), th.FolderIconGlyph(theme.FolderIconExcluded))
	}
}

func TestResolveFolderIconKindNonDirectory(t *testing.T) {
	file := localfs.Entry{Name: "readme.txt", Path: "/tmp/readme.txt", Type: localfs.EntryFile}
	_, ok := ResolveFolderIconKind(file, FolderIconContext{})
	if ok {
		t.Fatal("non-directory should return false")
	}
}
