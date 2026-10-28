//go:build windows

package filetrust

import "os"

func checkInfo(st os.FileInfo) string {
	if !st.Mode().IsRegular() {
		return ReasonNotRegular
	}
	return ""
}
