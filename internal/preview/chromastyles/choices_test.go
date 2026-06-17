package chromastyles

import (
	"testing"

	"github.com/alecthomas/chroma/v2/styles"
)

func TestChoicesListsAllRegistryStyles(t *testing.T) {
	choices := Choices()
	if len(choices) != len(styles.Names()) {
		t.Fatalf("len(Choices()) = %d, want %d", len(choices), len(styles.Names()))
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
	choices := Choices()
	if got := IndexOf(choices, "monokai"); got < 0 || choices[got].Name != "monokai" {
		t.Fatalf("IndexOf(monokai) = %d", got)
	}
	if got := IndexOf(choices, "not-a-style"); got != 0 {
		t.Fatalf("IndexOf(missing) = %d, want 0", got)
	}
}
