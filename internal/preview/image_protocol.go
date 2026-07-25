package preview

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// ResolveImageProtocol picks the terminal graphics protocol from config and the environment.
// environ is typically os.Getenv. Invalid/empty cfg is treated as auto.
func ResolveImageProtocol(cfg string, environ func(string) string) previewpanel.ImageProtocol {
	switch strings.ToLower(strings.TrimSpace(cfg)) {
	case config.PreviewImageProtocolSixel:
		return previewpanel.ImageProtocolSixel
	case config.PreviewImageProtocolKitty:
		return previewpanel.ImageProtocolKitty
	default:
		// auto (and unknown values)
		if environ == nil {
			return previewpanel.ImageProtocolSixel
		}
		prog := strings.ToLower(strings.TrimSpace(environ("TERM_PROGRAM")))
		if prog == "kitty" || prog == "ghostty" {
			return previewpanel.ImageProtocolKitty
		}
		term := strings.ToLower(strings.TrimSpace(environ("TERM")))
		// Ghostty's default TERM is xterm-ghostty (not a "ghostty…" prefix); TERM_PROGRAM
		// is often missing over SSH, so match the substring.
		if term == "xterm-kitty" || strings.Contains(term, "ghostty") {
			return previewpanel.ImageProtocolKitty
		}
		return previewpanel.ImageProtocolSixel
	}
}
