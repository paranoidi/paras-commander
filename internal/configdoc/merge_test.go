package configdoc

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleCanonicalDoc = `# widget registry — field reference
#
# Each widget is a [[entry]] block.
#
# --- end of documentation ---
`

const sampleCanonicalDocNoSentinel = `# widget registry — field reference
#
# Each widget is a [[entry]] block.
`

func TestSplitUserBodyEntriesOnly(t *testing.T) {
	body := []byte(`[[entry]]
name = "alpha"
description = "First widget"
`)
	got := SplitUserBody(body)
	if !bytes.Equal(got, body) {
		t.Fatalf("got %q, want %q", got, body)
	}
}

func TestSplitUserBodyOldDocPlusEntries(t *testing.T) {
	entries := `[[entry]]
name = "alpha"
description = "First widget"
`
	content := `# stale header
# outdated notes
` + entries
	got := SplitUserBody([]byte(content))
	if string(got) != entries {
		t.Fatalf("got %q, want %q", got, entries)
	}
}

func TestSplitUserBodyAllComments(t *testing.T) {
	content := `# only comments
#
# nothing configured yet
`
	got := SplitUserBody([]byte(content))
	if len(got) != 0 {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestSplitUserBodySentinel(t *testing.T) {
	entries := `[[entry]]
name = "beta"
`
	content := `# old doc
# --- end of documentation ---

` + entries + `
# note below config must stay
`
	got := SplitUserBody([]byte(content))
	want := "\n" + entries + "\n# note below config must stay\n"
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSplitUserBodySentinelWithConfigCommentAbove(t *testing.T) {
	entries := `shell_patterns = true
[[pools]]
name = "worker"
max_parallel = 4
`
	content := sampleCanonicalDoc + "\n" + entries
	got := SplitUserBody([]byte(content))
	// Leading newline after sentinel is trimmed during MergeDocumentation.
	if string(bytes.TrimLeft(got, "\n")) != entries {
		t.Fatalf("got %q, want body %q", got, entries)
	}
}

func TestMergeDocumentationPrependsDoc(t *testing.T) {
	body := []byte(`[[entry]]
name = "gamma"
`)
	got := MergeDocumentation(sampleCanonicalDocNoSentinel, body)
	want := sampleCanonicalDocNoSentinel + "\n" + string(body)
	if string(got) != want {
		t.Fatalf("merged:\n%s\nwant:\n%s", got, want)
	}
}

func TestMergeDocumentationReplacesDocPreservesEntries(t *testing.T) {
	entries := `[[entry]]
name = "delta"
description = "Saved row"
file = "wc -l"
`
	oldDoc := `# legacy title
# old field list
`
	content := oldDoc + entries
	got := MergeDocumentation(sampleCanonicalDocNoSentinel, SplitUserBody([]byte(content)))
	if !strings.HasSuffix(string(got), entries) {
		t.Fatalf("entries not preserved at suffix:\n%s", got)
	}
	if !strings.HasPrefix(string(got), sampleCanonicalDocNoSentinel+"\n") {
		t.Fatalf("canonical doc not at prefix:\n%s", got)
	}
}

func TestMergeDocumentationEmptyBody(t *testing.T) {
	got := MergeDocumentation(sampleCanonicalDocNoSentinel, nil)
	want := normalizeCanonicalDoc(sampleCanonicalDocNoSentinel)
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMergeDocumentationAlreadyCurrent(t *testing.T) {
	entries := `[[pools]]
name = "io"
max_parallel = 2
`
	merged := MergeDocumentation(sampleCanonicalDoc, []byte(entries))
	again := MergeDocumentation(sampleCanonicalDoc, SplitUserBody(merged))
	if !bytes.Equal(merged, again) {
		t.Fatalf("idempotent merge changed bytes")
	}
}

func TestRefreshDocumentationPrependsOnEntriesOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "widgets.toml")
	entries := `[[entry]]
name = "epsilon"
description = "Runnable"
`
	if err := os.WriteFile(path, []byte(entries), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := RefreshDocumentation(path, sampleCanonicalDocNoSentinel)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("want changed=true")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), sampleCanonicalDocNoSentinel) {
		t.Fatalf("missing canonical doc:\n%s", b)
	}
	if !strings.HasSuffix(string(b), entries) {
		t.Fatalf("entries not preserved:\n%s", b)
	}
}

func TestRefreshDocumentationNoWriteWhenCurrent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "widgets.toml")
	entries := `[[entry]]
name = "zeta"
`
	original := MergeDocumentation(sampleCanonicalDoc, []byte(entries))
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	infoBefore, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := RefreshDocumentation(path, sampleCanonicalDoc)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("want changed=false")
	}
	infoAfter, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !infoBefore.ModTime().Equal(infoAfter.ModTime()) {
		t.Fatal("file mod time changed despite no write")
	}
}

func TestRefreshDocumentationReplacesAllCommentStub(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "widgets.toml")
	stale := `# ancient header
#
# commented examples only
`
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := RefreshDocumentation(path, sampleCanonicalDoc)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("want changed=true")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != normalizeCanonicalDoc(sampleCanonicalDoc) {
		t.Fatalf("got:\n%s\nwant:\n%s", b, normalizeCanonicalDoc(sampleCanonicalDoc))
	}
}

func TestRefreshDocumentationEmptyPath(t *testing.T) {
	if _, err := RefreshDocumentation("", sampleCanonicalDoc); err == nil {
		t.Fatal("expected error for empty path")
	}
}
