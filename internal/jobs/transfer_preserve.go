package jobs

// TransferPreserve holds per-job copy/move metadata options set at enqueue time.
type TransferPreserve struct {
	PreservePermissions bool
	PreserveTimestamps  bool
	// FlattenIntoDest requests dest/<basename> naming (transfer-dialog "Flatten into destination").
	FlattenIntoDest bool
	// DereferenceSymlinks requests copying through symlinks instead of recreating them.
	// Copy only — AddTransferJob forces this false for Move regardless of this value.
	DereferenceSymlinks bool
	// ConflictRules presets the job's conflict rules (transfer-dialog Advanced); nil asks as usual.
	ConflictRules *ConflictRules
	// RemoveEmptyDirs removes the source directories a move leaves empty, without the prompt
	// (move only).
	RemoveEmptyDirs bool
}

// TransferPreserveFromConfig returns transfer preserve flags from operations config defaults.
func TransferPreserveFromConfig(preservePermissions, preserveTimestamps, dereferenceSymlinks bool) TransferPreserve {
	return TransferPreserve{
		PreservePermissions: preservePermissions,
		PreserveTimestamps:  preserveTimestamps,
		DereferenceSymlinks: dereferenceSymlinks,
	}
}
