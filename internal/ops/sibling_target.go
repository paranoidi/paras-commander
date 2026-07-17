package ops

import (
	"context"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// siblingTargetRules customize validation messages and checks for in-place rename/duplicate.
type siblingTargetRules struct {
	op            string
	sameNameText  string
	requireUTF8   bool
	rejectDotDot  bool
}

// planSiblingTarget resolves source and destination paths for a same-directory rename/duplicate.
func planSiblingTarget(sourcePath, panelPath, newName string, rules siblingTargetRules) (srcLoc, destLoc pathloc.Path, err error) {
	if newName == "" {
		return pathloc.Path{}, pathloc.Path{}, &Error{Op: rules.op, Text: "name is empty"}
	}
	if strings.Contains(newName, "/") || strings.Contains(newName, string(filepath.Separator)) {
		return pathloc.Path{}, pathloc.Path{}, &Error{Op: rules.op, Text: "name must be a single filename without path separators"}
	}
	if rules.requireUTF8 && !utf8.ValidString(newName) {
		return pathloc.Path{}, pathloc.Path{}, &Error{Op: rules.op, Text: "name is not valid UTF-8"}
	}
	if rules.rejectDotDot && (newName == "." || newName == "..") {
		return pathloc.Path{}, pathloc.Path{}, &Error{Op: rules.op, Text: "invalid name"}
	}

	srcLoc, err = pathloc.Parse(sourcePath)
	if err != nil {
		return pathloc.Path{}, pathloc.Path{}, &Error{Op: rules.op, Text: "invalid source path", Err: err}
	}
	if srcLoc.Base() == newName {
		return pathloc.Path{}, pathloc.Path{}, &Error{Op: rules.op, Text: rules.sameNameText}
	}

	parent := srcLoc.Parent()
	if parent.IsZero() {
		panel, perr := pathloc.Parse(panelPath)
		if perr != nil {
			return pathloc.Path{}, pathloc.Path{}, &Error{Op: rules.op, Text: "invalid panel path", Err: perr}
		}
		parent = panel
	}
	destLoc, err = parent.Join(newName)
	if err != nil {
		return pathloc.Path{}, pathloc.Path{}, &Error{Op: rules.op, Text: err.Error(), Err: err}
	}

	if _, err := statEntry(context.Background(), destLoc); err == nil {
		return pathloc.Path{}, pathloc.Path{}, &Error{Op: rules.op, Text: "target already exists"}
	} else if !isNotExist(err) {
		return pathloc.Path{}, pathloc.Path{}, &Error{Op: rules.op, Text: "cannot stat target", Err: err}
	}
	return srcLoc, destLoc, nil
}
