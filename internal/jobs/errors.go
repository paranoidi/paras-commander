package jobs

import "errors"

// ErrUserCanceled is returned when the user aborts a blocker (file conflict or disk space).
var ErrUserCanceled = errors.New("canceled by user")
