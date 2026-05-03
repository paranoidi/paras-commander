//go:build windows

package diskusage

func pathStatDevice(string) (uint64, bool) {
	return 0, false
}

// PathDevice returns st_dev for abs on Unix; on Windows ok is always false.
func PathDevice(abs string) (dev uint64, ok bool) {
	if abs == "" {
		return 0, false
	}
	return 0, false
}
