package localfs

// CopyFileOpts configures optional local copy behavior (buffer reuse, kernel fast paths, sparse/preallocate).
type CopyFileOpts struct {
	Buf            []byte
	CopyFileRange  bool
	SparseCopy     bool
	Preallocate    bool
	PreallocateMin int64
	SyncPerFile    bool
	SyncMinFileKiB int
}

func (o CopyFileOpts) syncNow(size int64) bool {
	if !o.SyncPerFile {
		return false
	}
	if o.SyncMinFileKiB <= 0 {
		return true
	}
	return size >= int64(o.SyncMinFileKiB)*1024
}

func (o CopyFileOpts) shouldPreallocate(size int64) bool {
	if !o.Preallocate || size <= 0 {
		return false
	}
	if o.PreallocateMin <= 0 {
		return true
	}
	return size >= o.PreallocateMin
}
