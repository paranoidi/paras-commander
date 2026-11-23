package theme

import "testing"

func TestPanelTabCaps(t *testing.T) {
	th := Default()
	if start, end := th.PanelTabCaps(); start != " " || end != " " {
		t.Fatalf("without Nerd Font icons caps = %q %q, want square", start, end)
	}
	th.UseNerdfontIcons = true
	if start, end := th.PanelTabCaps(); start != "\ue0ba " || end != " \ue0b8" {
		t.Fatalf("default caps = %q %q, want the slanted pair with their padding", start, end)
	}
}

func TestPanelTabCapsFromThemeFile(t *testing.T) {
	th, err := parse([]byte("name = \"caps\"\n[icons.panel.tab]\nstart = \"\\ue0b6 \"\nend = \" \\ue0b4\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	th.UseNerdfontIcons = true
	if start, end := th.PanelTabCaps(); start != "\ue0b6 " || end != " \ue0b4" {
		t.Fatalf("pill caps = %q %q", start, end)
	}
	square, err := parse([]byte("name = \"sq\"\n[icons.panel.tab]\nstart = \" \"\nend = \" \"\n"))
	if err != nil {
		t.Fatal(err)
	}
	square.UseNerdfontIcons = true
	if start, end := square.PanelTabCaps(); start != " " || end != " " {
		t.Fatalf("square caps = %q %q (untrimmed)", start, end)
	}
}
