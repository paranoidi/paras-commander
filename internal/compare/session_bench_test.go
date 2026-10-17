package compare

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

var benchWords = []string{"apple", "river", "stone", "cloud", "maple", "ember", "frost", "harbor", "lantern", "meadow", "orchid", "pebble", "quartz", "willow", "zephyr", "thistle"}

// TestSessionHashBench times a full compare of two identical trees. Opt-in:
//
//	PC_COMPARE_BENCH=1 [PC_COMPARE_BENCH_FILES=5000] [PC_COMPARE_BENCH_WORKERS=4] go test ./internal/compare -run SessionHashBench -v
func TestSessionHashBench(t *testing.T) {
	if os.Getenv("PC_COMPARE_BENCH") == "" {
		t.Skip("set PC_COMPARE_BENCH=1 to run")
	}
	envInt := func(k string, def int) int {
		if v, err := strconv.Atoi(os.Getenv(k)); err == nil && v > 0 {
			return v
		}
		return def
	}
	files := envInt("PC_COMPARE_BENCH_FILES", 5000)
	workers := envInt("PC_COMPARE_BENCH_WORKERS", 4)

	a, b := t.TempDir(), t.TempDir()
	rng := rand.New(rand.NewSource(1))
	var total int64
	for i := 0; i < files; i++ {
		rel := filepath.Join(
			fmt.Sprintf("%s-%d", benchWords[rng.Intn(len(benchWords))], i%40),
			fmt.Sprintf("%s-%s-%d.dat", benchWords[rng.Intn(len(benchWords))], benchWords[rng.Intn(len(benchWords))], i))
		data := make([]byte, 100_000+rng.Intn(200_000))
		rng.Read(data)
		for _, root := range []string{a, b} {
			p := filepath.Join(root, rel)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		total += int64(len(data)) * 2
	}

	start := time.Now()
	sess := Start(context.Background(), mustFilePath(t, a), mustFilePath(t, b), Options{
		HashWorkers: workers,
		ReadBuffer:  make([]byte, 256*1024),
	})
	defer sess.Close()
	for {
		s := sess.Snapshot()
		if s.Phase == PhaseDone {
			break
		}
		if s.Phase == PhaseError || s.Phase == PhaseCanceled {
			t.Fatalf("phase %v: %s", s.Phase, s.Err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	el := time.Since(start)
	t.Logf("files=%d per side, bytes=%d, workers=%d, elapsed=%s, %.1f MB/s", files, total, workers, el, float64(total)/1e6/el.Seconds())
}
