//go:build !unix

package editor

func needsWriteElevation(string) bool { return false }
