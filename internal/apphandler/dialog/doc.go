// Package dialog will host modal dialog controllers (file dialogs, path picker, transfer).
// File-dialog orchestration currently remains in internal/app (dialog_*.go); path picker
// and transfer destination validation still use App-owned debounce timers until migrated here.
package dialog
