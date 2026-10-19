package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUeberzugCommandLines(t *testing.T) {
	path := `/tmp/quiet "harbor"\garden.png`
	line := ueberzugAddLine(3, 4, 20, 10, path)
	if !strings.HasSuffix(string(line), "\n") {
		t.Fatalf("missing newline: %q", line)
	}
	var got map[string]any
	if err := json.Unmarshal(line, &got); err != nil {
		t.Fatalf("invalid JSON %q: %v", line, err)
	}
	want := map[string]any{"action": "add", "identifier": "pc", "x": 3.0, "y": 4.0, "max_width": 20.0, "max_height": 10.0, "path": path}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s = %v, want %v", k, got[k], v)
		}
	}
	if string(ueberzugRemoveLine()) != `{"action":"remove","identifier":"pc"}`+"\n" {
		t.Fatalf("remove line = %q", ueberzugRemoveLine())
	}
}
