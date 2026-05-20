package menu

import "testing"

func TestDevDefinitionShortcuts(t *testing.T) {
	def := DevDefinition()
	if def.Shortcut != 'v' {
		t.Fatalf("Dev menu shortcut = %q, want v", def.Shortcut)
	}
	want := map[string]rune{
		"Show info":  's',
		"Show warn":  'w',
		"Show error": 'e',
	}
	for _, item := range def.Items {
		got, ok := want[item.Label]
		if !ok {
			t.Fatalf("unexpected item %q", item.Label)
		}
		if item.Shortcut != got {
			t.Fatalf("%q shortcut = %q, want %q", item.Label, item.Shortcut, got)
		}
	}
}

func TestBrowserDefinitionsOmitsDevByDefault(t *testing.T) {
	defs := BrowserDefinitions(nil, false)
	for _, def := range defs {
		if def.ID == TopDev {
			t.Fatal("browser menus must not include Dev without dev flag")
		}
	}
}

func TestBrowserDefinitionsIncludesDevWhenEnabled(t *testing.T) {
	defs := BrowserDefinitions(nil, true)
	found := false
	for _, def := range defs {
		if def.ID == TopDev {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("browser menus with dev=true must include Dev pulldown")
	}
}
