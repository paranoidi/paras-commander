package compare

import "errors"

var errRemoteNotSupported = errors.New("compare: remote paths not supported yet")

// ErrRemoteUnsupported reports remote compare is not implemented.
func ErrRemoteUnsupported() error { return errRemoteNotSupported }
