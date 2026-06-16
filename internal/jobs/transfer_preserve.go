package jobs

// TransferPreserve holds per-job copy/move metadata options set at enqueue time.
type TransferPreserve struct {
	PreservePermissions bool
	PreserveTimestamps  bool
}

// FromConfig returns transfer preserve flags from operations config defaults.
func TransferPreserveFromConfig(preservePermissions, preserveTimestamps bool) TransferPreserve {
	return TransferPreserve{
		PreservePermissions: preservePermissions,
		PreserveTimestamps:  preserveTimestamps,
	}
}
