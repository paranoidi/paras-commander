//go:build windows

package usermenu

import "os"

func fileOwnerTrusted(_ os.FileInfo) bool {
	return true
}
