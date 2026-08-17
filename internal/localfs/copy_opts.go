package localfs

// CopyFileOpts configures optional local copy behavior (buffer reuse, kernel fast paths, sparse/preallocate).
type CopyFileOpts struct {
	Buf []byte
	// CopyFileRange enables the copy_file_range(2) fast path.
	CopyFileRange bool
	// CopyFileRangeChunkBytes caps each copy_file_range(2) syscall so cancellation
	// (checked between chunks) responds promptly on large files. <= 0 falls back
	// to the syscall's own max (math.MaxInt32).
	CopyFileRangeChunkBytes int64
	SparseCopy              bool
	Preallocate             bool
	PreallocateMin          int64
	SyncPerFile             bool
	SyncMinFileKiB          int
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
