//go:build windows

package metacmds

import "os"

func fileOwnerTrusted(_ os.FileInfo) bool {
	return true
}
