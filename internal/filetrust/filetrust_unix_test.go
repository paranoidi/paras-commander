//go:build !windows

package filetrust

import (
	"os"
	"syscall"
	"testing"
)

// checkInfo must see the real owner: a failed Sys() assertion silently skips the owner rule.
func TestCheckInfo_sysIsStatT(t *testing.T) {
	st, err := os.Lstat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Sys().(*syscall.Stat_t); !ok {
		t.Fatalf("Sys() = %T, want *syscall.Stat_t", st.Sys())
	}
}
