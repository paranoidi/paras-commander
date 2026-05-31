package pools

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
)

func TestDecodeNormalizesPools(t *testing.T) {
	mf, err := Decode([]byte(`[[pools]]
name = "a"
max_parallel = 0

[[pools]]
name = "b"
max_parallel = 999
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(mf.Pools) != 2 {
		t.Fatalf("Pools len = %d, want 2", len(mf.Pools))
	}
	if mf.Pools[0].Name != "a" || mf.Pools[0].MaxParallel != 1 {
		t.Fatalf("first pool = %+v, want a/1", mf.Pools[0])
	}
	if mf.Pools[1].Name != "b" || mf.Pools[1].MaxParallel != config.DefaultPoolMaxParallel {
		t.Fatalf("second pool = %+v, want b/%d", mf.Pools[1], config.DefaultPoolMaxParallel)
	}
}

func TestDecodeDuplicatePoolName(t *testing.T) {
	_, err := Decode([]byte(`[[pools]]
name = "a"
max_parallel = 1

[[pools]]
name = "a"
max_parallel = 2
`))
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("Decode() err = %v, want duplicate name error", err)
	}
}

func TestDecodeEmptyPoolName(t *testing.T) {
	_, err := Decode([]byte(`[[pools]]
name = ""
max_parallel = 1
`))
	if err == nil {
		t.Fatal("expected error for empty pool name")
	}
}

func TestDecodeEmptyFile(t *testing.T) {
	mf, err := Decode([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(mf.Pools) != 0 {
		t.Fatalf("Pools len = %d, want 0", len(mf.Pools))
	}
}
