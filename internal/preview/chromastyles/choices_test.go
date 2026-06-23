package chromastyles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/chroma/v2/styles"
)

func TestChoicesListsBuiltInAndCustomStyles(t *testing.T) {
	t.Cleanup(ResetForTest)
	choices := Choices()
	if len(choices) != len(styles.Names()) {
		t.Fatalf("len(Choices()) = %d, want %d built-in only", len(choices), len(styles.Names()))
	}
	for i, c := range choices {
		if c.Name == "" || c.Label != c.Name {
			t.Fatalf("choice[%d] = %+v, want Name==Label", i, c)
		}
		if i > 0 && choices[i-1].Name >= c.Name {
			t.Fatalf("choices not sorted: %q before %q", choices[i-1].Name, c.Name)
		}
	}
	for _, c := range choices {
		if c.Name == "default" {
			t.Fatal("app UI theme name default must not appear in chroma style list")
		}
	}
}

func TestIndexOf(t *testing.T) {
	t.Cleanup(ResetForTest)
	choices := Choices()
	if got := IndexOf(choices, "monokai"); got < 0 || choices[got].Name != "monokai" {
		t.Fatalf("IndexOf(monokai) = %d", got)
	}
	if got := IndexOf(choices, "not-a-style"); got != 0 {
		t.Fatalf("IndexOf(missing) = %d, want 0", got)
	}
}

func TestChoicesIncludesCustomAfterLoad(t *testing.T) {
	t.Cleanup(ResetForTest)
	dir := t.TempDir()
	xml := `<style name="violet-harbor-preview"><entry type="Keyword" style="#aabbcc"/></style>`
	if err := os.WriteFile(filepath.Join(dir, "violet-harbor-preview.xml"), []byte(xml), 0o644); err != nil {
		t.Fatalf("write style: %v", err)
	}
	if err := LoadFromDir(dir); err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if got := IndexOf(Choices(), "violet-harbor-preview"); Choices()[got].Name != "violet-harbor-preview" {
		t.Fatalf("custom style missing from Choices(), IndexOf = %d", got)
	}
}
