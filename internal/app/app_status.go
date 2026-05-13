package app

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
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
	if strings.TrimSpace(msg) == "" {
		a.clearTransientMessage()
		return
	}
	gen := a.messageExpiryGen.Add(1)
	a.model.Message = msg
	a.model.MessageUrgency = urgency
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
	a.setTransientMessage(fmt.Sprintf("%s: %v", prefix, err), ui.MessageUrgencyError)
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
