package localfs

import "os"

// abortPartialLocalCopy closes an open destination handle and removes a partial file
// after a content-copy, sync, or close failure.
func abortPartialLocalCopy(dstFile *os.File, target string) {
	if dstFile != nil {
		_ = dstFile.Close()
	}
	_ = os.Remove(target)
}
