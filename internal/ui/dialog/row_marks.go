package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// RowMarks is the pin / in-progress-job / missing state of one list row's trailing suffix icons;
// the type, its width (dlg.RowMarksWidth) and its painter (dlg.DrawRowMarksSuffix) live in the dlg
// engine, which draws them for list rows.
type RowMarks = dlg.RowMarks

// RowMarksFunc resolves RowMarks for one row's absolute path.
type RowMarksFunc func(absPath string) RowMarks
