package localfs

// CopyFileOpts configures optional local copy behavior (buffer reuse, kernel fast paths, sparse/preallocate).
type CopyFileOpts struct {
	Buf              []byte
	CopyFileRange    bool
	SparseCopy       bool
	Preallocate      bool
	PreallocateMin   int64
}
