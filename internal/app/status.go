package app

import (
	"errors"
	"fmt"
	"io/fs"
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

func (a *App) setTransientMessage(msg string, urgency ui.MessageUrgency) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		a.clearTransientMessage()
		return
	}
	lines := ui.WrapWordsToWidth(msg, ui.MessageLogWrapRunes)
	if len(lines) == 0 {
		a.clearTransientMessage()
		return
	}
	gen := a.messageExpiryGen.Add(1)
	a.model.Message = lines[0]
	a.model.MessageUrgency = urgency
	a.appendMessageLog(msg, urgency)
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

func (a *App) appendMessageLog(text string, urgency ui.MessageUrgency) {
	t := strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if t == "" {
		return
	}
	lines := ui.WrapWordsToWidth(t, ui.MessageLogWrapRunes)
	if len(lines) == 0 {
		return
	}
	max := a.config.UI.MessageLogMaxEntries
	if max <= 0 {
		max = config.DefaultMessageLogMaxEntries
	}
	wasAtEnd := a.model.ViewMode == ui.ViewMessages && len(a.model.MessageLog) > 0 &&
		a.model.MessagesView.Selected == len(a.model.MessageLog)-1

	ts := time.Now().Format("15:04:05")
	for i, line := range lines {
		tm := ""
		if i == 0 {
			tm = ts
		}
		a.model.MessageLog = append(a.model.MessageLog, ui.MessageLogEntry{
			Time: tm,
			Text: line,
			Urg:  urgency,
		})
	}
	for len(a.model.MessageLog) > max {
		a.model.MessageLog = a.model.MessageLog[1:]
		if a.model.MessagesView.Selected > 0 {
			a.model.MessagesView.Selected--
		}
		if a.model.MessagesView.ListScroll > 0 {
			a.model.MessagesView.ListScroll--
		}
	}
	if wasAtEnd && len(a.model.MessageLog) > 0 {
		a.model.MessagesView.Selected = len(a.model.MessageLog) - 1
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

// jobFailureBannerDetail returns short text for the status banner on job failure.
// Full detail remains on the job record (jobs panel).
func jobFailureBannerDetail(err error, fallback string) string {
	if err != nil {
		if short := transientErrorText(err); short != err.Error() {
			return short
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
