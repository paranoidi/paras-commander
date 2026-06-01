package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) clearTransientMessage() {
	a.messageExpiryGen.Add(1)
	a.model.Message = ""
	a.model.MessageUrgency = ui.MessageUrgencyInfo
}

func (a *App) statusMessageTTL() time.Duration {
	sec := a.config.UI.StatusMessageTTLSeconds
	if sec <= 0 {
		return 0
	}
	return time.Duration(sec * float64(time.Second))
}

func (a *App) messageLogWrapCols() int {
	w, h := a.screen.Size()
	if w <= 0 || h <= 0 {
		return ui.MessageLogWrapRunes
	}
	return ui.MessageLogWrapColsForLayout(a.layoutForTerminalSize(w, h))
}

// statusMessageWrapCols is the max runes for one status-banner line (full terminal width).
func (a *App) statusMessageWrapCols() int {
	w, _ := a.screen.Size()
	if w < 1 {
		return ui.MessageLogWrapRunes
	}
	// drawStatusMessageOverlay applies FormatToastDisplay (+2 runes) then clips to rect.Width.
	if w > 2 {
		return w - 2
	}
	return 1
}

func toastWrapLines(msg string, maxCols int) []string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return nil
	}
	if maxCols < 1 {
		maxCols = ui.MessageLogWrapRunes
	}
	return ui.WrapWordsToWidth(msg, maxCols)
}

func (a *App) setTransientMessage(msg string, urgency ui.MessageUrgency) {
	a.setTransientMessageBanner(msg, "", urgency)
}

// setTransientMessageBanner logs logMsg (wrapped to the Messages column width) and shows bannerMsg
// on the status banner when non-empty, otherwise the first wrapped log line.
func (a *App) setTransientMessageBanner(logMsg, bannerMsg string, urgency ui.MessageUrgency) {
	lines := toastWrapLines(logMsg, a.messageLogWrapCols())
	if len(lines) == 0 {
		a.clearTransientMessage()
		return
	}
	banner := strings.TrimSpace(bannerMsg)
	if banner == "" {
		bannerLines := toastWrapLines(logMsg, a.statusMessageWrapCols())
		if len(bannerLines) > 0 {
			banner = bannerLines[0]
		}
	}
	gen := a.messageExpiryGen.Add(1)
	a.model.Message = banner
	a.model.MessageUrgency = urgency
	a.appendMessageLogLines(lines, urgency)
	ttl := a.statusMessageTTL()
	if ttl <= 0 {
		return
	}
	go func(g uint64, d time.Duration) {
		time.Sleep(d)
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(statusMessageExpiryPayload{gen: g}))
	}(gen, ttl)
}

func (a *App) applyStatusMessageExpiry(p statusMessageExpiryPayload) {
	if p.gen != a.messageExpiryGen.Load() {
		return
	}
	if strings.TrimSpace(a.model.Message) == "" {
		return
	}
	a.model.Message = ""
	a.model.MessageUrgency = ui.MessageUrgencyInfo
}

func (a *App) appendMessageLogLines(lines []string, urgency ui.MessageUrgency) {
	if len(lines) == 0 {
		return
	}
	max := a.config.UI.MessageLogMaxEntries
	if max <= 0 {
		max = config.DefaultMessageLogMaxEntries
	}
	wasAtTop := a.model.ViewMode == ui.ViewMessages && len(a.model.MessageLog) > 0 &&
		a.model.MessagesView.Selected == 0

	ts := time.Now().Format("15:04:05")
	batch := make([]ui.MessageLogEntry, len(lines))
	for i, line := range lines {
		tm := ""
		if i == 0 {
			tm = ts
		}
		batch[i] = ui.MessageLogEntry{
			Time: tm,
			Text: line,
			Urg:  urgency,
		}
	}
	a.model.MessageLog = append(batch, a.model.MessageLog...)
	if !wasAtTop && len(batch) > 0 {
		a.model.MessagesView.Selected++
	}
	for len(a.model.MessageLog) > max {
		a.model.MessageLog = a.model.MessageLog[:max]
	}
	if wasAtTop && len(a.model.MessageLog) > 0 {
		a.model.MessagesView.Selected = 0
		a.model.MessagesView.ListScroll = 0
	}
	a.ensureMessagesViewSelectionVisible()
}

func (a *App) setUnsupportedMessage(label string) {
	a.setTransientMessage(label+" is not implemented yet", ui.MessageUrgencyWarn)
}

func (a *App) setErrorMessage(prefix string, err error) {
	if err == nil {
		return
	}
	if prefix == "Enter failed" {
		if msg, ok := enterFailedMessage(err); ok {
			a.setTransientMessage(msg, ui.MessageUrgencyError)
			return
		}
	}
	if short := transientErrorText(err); short != err.Error() {
		a.setTransientMessage(short, ui.MessageUrgencyError)
		return
	}
	if shouldOmitErrorPrefix(prefix, err) {
		a.setTransientMessage(err.Error(), ui.MessageUrgencyError)
		return
	}
	a.setTransientMessage(fmt.Sprintf("%s: %v", prefix, err), ui.MessageUrgencyError)
}

// enterFailedMessage formats missing-directory enter failures as:
// Enter failed: no such directory "/path"
func enterFailedMessage(err error) (string, bool) {
	if err == nil || !errors.Is(err, fs.ErrNotExist) {
		return "", false
	}
	path := quotedPathFromError(err)
	if path == "" {
		return "", false
	}
	return fmt.Sprintf(`Enter failed: no such directory %q`, path), true
}

func quotedPathFromError(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && pathErr.Path != "" {
		return pathErr.Path
	}
	msg := err.Error()
	for _, prefix := range []string{`stat directory "`, `read directory "`, `resolve directory "`} {
		if rest, ok := strings.CutPrefix(msg, prefix); ok {
			if path, _, ok := strings.Cut(rest, `"`); ok && path != "" {
				return path
			}
		}
		if i := strings.Index(msg, prefix); i >= 0 {
			rest := msg[i+len(prefix):]
			if path, _, ok := strings.Cut(rest, `"`); ok && path != "" {
				return path
			}
		}
	}
	return ""
}

// shouldOmitErrorPrefix reports whether err.Error() already identifies the operation
// named in prefix (e.g. ops.Error, localfs mkdir/rename wrappers, mass-rename stages).
func shouldOmitErrorPrefix(prefix string, err error) bool {
	var opErr *ops.Error
	if errors.As(err, &opErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, verb := range errorPrefixVerbs(prefix) {
		if strings.HasPrefix(msg, verb) {
			return true
		}
	}
	return false
}

func errorPrefixVerbs(prefix string) []string {
	p := strings.TrimSpace(strings.ToLower(prefix))
	p = strings.TrimSuffix(p, " failed")
	p = strings.TrimSuffix(p, " source")
	switch p {
	case "mkdir":
		return []string{"mkdir"}
	case "rename":
		return []string{"rename"}
	case "chmod":
		return []string{"chmod"}
	case "chown":
		return []string{"chown"}
	case "symlink":
		return []string{"symlink"}
	case "hardlink":
		return []string{"hardlink", "link"}
	case "mass rename":
		return []string{"mass rename", "mass-rename"}
	case "delete":
		return []string{"delete", "remove"}
	default:
		return nil
	}
}

// transientErrorText maps common filesystem errors to compact status-banner text.
func transientErrorText(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, fs.ErrPermission) {
		return "permission denied"
	}
	return err.Error()
}

const jobFailureBannerMaxRunes = 72

// jobFailureLogDetail returns full text for the Messages log on job failure.
func jobFailureLogDetail(err error, fallback string) string {
	if err != nil {
		return firstMessageLine(err.Error())
	}
	return firstMessageLine(fallback)
}

// jobFailureBannerDetail returns short text for the status banner on job failure.
func jobFailureBannerDetail(err error, fallback string) string {
	if err != nil {
		if short := transientErrorText(err); short != err.Error() {
			return short
		}
		var opErr *ops.Error
		if errors.As(err, &opErr) && opErr.Err != nil {
			if inner := ops.NestedErrorText(opErr.Err); inner != "" {
				if short := transientErrorText(errors.New(inner)); short != inner {
					return short
				}
				return truncateStatusBannerRunes(inner, jobFailureBannerMaxRunes)
			}
		}
		return truncateStatusBannerRunes(firstMessageLine(err.Error()), jobFailureBannerMaxRunes)
	}
	line := firstMessageLine(fallback)
	if strings.Contains(strings.ToLower(line), "permission denied") {
		return "permission denied"
	}
	return truncateStatusBannerRunes(line, jobFailureBannerMaxRunes)
}

func truncateStatusBannerRunes(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if maxRunes <= 0 || s == "" {
		return s
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return strings.TrimSpace(string(r[:maxRunes])) + "…"
}

// firstMessageLine returns the first non-empty line of s (after trim), for errors that join
// multiple messages with newlines.
func firstMessageLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	for _, part := range strings.Split(s, "\n") {
		line := strings.TrimSpace(part)
		if line != "" {
			return line
		}
	}
	return s
}
