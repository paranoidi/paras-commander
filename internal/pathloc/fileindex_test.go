package pathloc

import "testing"

func TestFileIndexPathRules(t *testing.T) {
	p := MustParse("fileindex:///orchard/lantern//meadow.txt")
	if p.Scheme() != SchemeFileIndex || p.IsLocal() {
		t.Fatalf("scheme = %s remote = %v", p.Scheme(), !p.IsLocal())
	}
	if p.String() != "fileindex:///orchard/lantern/meadow.txt" {
		t.Fatalf("String = %q, want the cleaned host path", p.String())
	}
	if p.Base() != "meadow.txt" {
		t.Fatalf("Base = %q", p.Base())
	}
	if got := p.Parent().String(); got != "fileindex:///orchard/lantern" {
		t.Fatalf("Parent = %q", got)
	}
	root := MustParse("fileindex:///")
	if !root.Parent().Equal(root) {
		t.Fatalf("root Parent = %q, want itself", root.Parent())
	}
	if root.Base() != "/" {
		t.Fatalf("root Base = %q", root.Base())
	}
	if !p.HasPrefix(MustParse("fileindex:///orchard")) || p.HasPrefix(MustParse("fileindex:///orch")) {
		t.Fatal("HasPrefix must follow file-scheme segment rules")
	}
	if p.HasPrefix(MustParse("/orchard")) || TreesOverlap(p, MustParse("/orchard/lantern")) {
		t.Fatal("a fileindex path never shares a tree with the host path it names")
	}
	child, err := MustParse("fileindex:///orchard").Join("thistle")
	if err != nil || child.String() != "fileindex:///orchard/thistle" {
		t.Fatalf("Join = %q, %v", child, err)
	}
	if host, err := IndexHostPath(p); err != nil || host != "/orchard/lantern/meadow.txt" {
		t.Fatalf("IndexHostPath = %q, %v", host, err)
	}
	if _, err := p.FilePath(); err == nil {
		t.Fatal("FilePath must refuse a fileindex path")
	}
	if loc, err := FileIndex("/harbor"); err != nil || loc.String() != "fileindex:///harbor" {
		t.Fatalf("FileIndex = %q, %v", loc, err)
	}
	if _, err := Parse("fileindex://harbor"); err == nil {
		t.Fatal("a relative fileindex path must be rejected")
	}
	if !HasURIPrefix("fileindex:///x") || !HasURIPrefix("sftp://h/x") || HasURIPrefix("/x") {
		t.Fatal("HasURIPrefix")
	}
}
