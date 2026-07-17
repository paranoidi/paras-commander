package ops

import "fmt"

// resolveOverwriteDecision asks the conflict resolver whether to overwrite an existing destination.
// Returns proceed=false when the user skips; an error when canceling or when no resolver is set.
func resolveOverwriteDecision(src, dst string, resolver ConflictResolver, facts FileConflictFacts) (proceed bool, err error) {
	if resolver == nil {
		return false, fmt.Errorf("destination %q already exists and no conflict resolver configured", dst)
	}
	overwrite, err := resolver(src, dst, facts)
	if err != nil {
		return false, err
	}
	return overwrite, nil
}
