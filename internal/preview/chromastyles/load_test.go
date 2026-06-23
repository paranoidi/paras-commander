package chromastyles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
)

const testCustomStyleXML = `<style name="amber-bloom-preview">
  <entry type="Background" style="bg:#1a1a1a #eeeeee"/>
  <entry type="Keyword" style="#ff00ff"/>
</style>`

const testOverrideMonokaiXML = `<style name="monokai">
  <entry type="Background" style="bg:#000000 #ffffff"/>
  <entry type="Keyword" style="#010101"/>
</style>`

func TestLoadFromDirParsesValidXML(t *testing.T) {
	t.Cleanup(ResetForTest)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "amber-bloom-preview.xml"), []byte(testCustomStyleXML), 0o644); err != nil {
		t.Fatalf("write style: %v", err)
	}
	if err := LoadFromDir(dir); err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if !IsValid("amber-bloom-preview") {
		t.Fatal("IsValid(amber-bloom-preview) = false, want true")
	}
	style := Get("amber-bloom-preview")
	if style == nil {
		t.Fatal("Get returned nil style")
	}
	entry := style.Get(chroma.Keyword)
	if !entry.Colour.IsSet() {
		t.Fatal("custom Keyword color not set")
	}
}

func TestLoadFromDirSkipsInvalidSibling(t *testing.T) {
	t.Cleanup(ResetForTest)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.xml"), []byte("not xml"), 0o644); err != nil {
		t.Fatalf("write broken: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "amber-bloom-preview.xml"), []byte(testCustomStyleXML), 0o644); err != nil {
		t.Fatalf("write style: %v", err)
	}
	if err := LoadFromDir(dir); err == nil {
		t.Fatal("LoadFromDir = nil, want warning error")
	}
	if len(LoadWarnings()) == 0 {
		t.Fatal("LoadWarnings empty, want broken.xml warning")
	}
	if !IsValid("amber-bloom-preview") {
		t.Fatal("valid sibling style not loaded")
	}
}

func TestLoadFromDirRejectsDefaultName(t *testing.T) {
	t.Cleanup(ResetForTest)
	dir := t.TempDir()
	xml := `<style name="default"><entry type="Keyword" style="#ff0000"/></style>`
	if err := os.WriteFile(filepath.Join(dir, "default.xml"), []byte(xml), 0o644); err != nil {
		t.Fatalf("write style: %v", err)
	}
	if err := LoadFromDir(dir); err == nil {
		t.Fatal("LoadFromDir = nil, want reserved-name error")
	}
	if IsValid("default") {
		t.Fatal("reserved default style must not register")
	}
}

func TestCustomStyleAppearsInChoices(t *testing.T) {
	t.Cleanup(ResetForTest)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "amber-bloom-preview.xml"), []byte(testCustomStyleXML), 0o644); err != nil {
		t.Fatalf("write style: %v", err)
	}
	if err := LoadFromDir(dir); err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	found := false
	for _, c := range Choices() {
		if c.Name == "amber-bloom-preview" {
			found = true
			if c.Label != "amber-bloom-preview" {
				t.Fatalf("Label = %q, want amber-bloom-preview", c.Label)
			}
		}
	}
	if !found {
		t.Fatal("custom style missing from Choices()")
	}
}

func TestCustomOverrideBuiltInWinsInGet(t *testing.T) {
	t.Cleanup(ResetForTest)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "monokai.xml"), []byte(testOverrideMonokaiXML), 0o644); err != nil {
		t.Fatalf("write style: %v", err)
	}
	if err := LoadFromDir(dir); err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	builtin := styles.Get("monokai")
	custom := Get("monokai")
	if custom == builtin {
		t.Fatal("custom override should replace built-in monokai")
	}
	entry := custom.Get(chroma.Keyword)
	if entry.Colour.String() != "#010101" {
		t.Fatalf("Keyword color = %q, want #010101", entry.Colour.String())
	}
}

func TestChoicesIncludesBuiltInAndCustomUnion(t *testing.T) {
	t.Cleanup(ResetForTest)
	before := len(Choices())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "amber-bloom-preview.xml"), []byte(testCustomStyleXML), 0o644); err != nil {
		t.Fatalf("write style: %v", err)
	}
	if err := LoadFromDir(dir); err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	after := len(Choices())
	if after != before+1 {
		t.Fatalf("len(Choices()) = %d, want %d", after, before+1)
	}
}

func TestCanonicalName(t *testing.T) {
	t.Cleanup(ResetForTest)
	if got := CanonicalName("Monokai"); got != "monokai" {
		t.Fatalf("CanonicalName(Monokai) = %q, want monokai", got)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "amber-bloom-preview.xml"), []byte(testCustomStyleXML), 0o644); err != nil {
		t.Fatalf("write style: %v", err)
	}
	if err := LoadFromDir(dir); err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if got := CanonicalName("AMBER-BLOOM-PREVIEW"); got != "amber-bloom-preview" {
		t.Fatalf("CanonicalName custom = %q, want amber-bloom-preview", got)
	}
}

func TestLoadFromDirMissingDirIsNoError(t *testing.T) {
	t.Cleanup(ResetForTest)
	if err := LoadFromDir(filepath.Join(t.TempDir(), "missing-preview-styles")); err != nil {
		t.Fatalf("missing dir LoadFromDir = %v, want nil", err)
	}
}

func TestLoadFromDirDuplicateNameWarns(t *testing.T) {
	t.Cleanup(ResetForTest)
	dir := t.TempDir()
	for _, name := range []string{"first.xml", "second.xml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(testCustomStyleXML), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	err := LoadFromDir(dir)
	if err == nil {
		t.Fatal("LoadFromDir = nil, want duplicate-name warning")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error = %v, want duplicate mention", err)
	}
}
