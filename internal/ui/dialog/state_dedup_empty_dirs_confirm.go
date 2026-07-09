package dialog

// DedupEmptyDirsConfirmState holds the "remove directories left empty by this
// delete?" confirmation shown after confirming a dedup delete.
type DedupEmptyDirsConfirmState struct {
	Open  bool
	Focus int // 0=Yes (default), 1=No
}
