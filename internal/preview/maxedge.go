package preview

import (
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// ImageMaxEdge returns the configured general still-image longest-edge clamp. 0 means
// unrestricted (decode at native resolution, subject only to the decode-megapixel safety net).
func ImageMaxEdge(cfg config.PreviewConfig) int {
	return cfg.ImageMaxEdgePx
}

// TmuxSixelMaxEdge returns the configured Sixel-under-tmux payload-safety clamp, or the
// built-in default. Unlike ImageMaxEdge, 0/unset always falls back to a concrete default —
// this clamp guards against a real silent-corruption bug and isn't meant to be foot-gunnable
// to "off".
func TmuxSixelMaxEdge(cfg config.PreviewConfig) int {
	if cfg.TmuxSixelMaxEdgePx < 1 {
		return config.DefaultPreviewTmuxSixelMaxEdgePx
	}
	return cfg.TmuxSixelMaxEdgePx
}

// VideoThumbMaxEdge returns the video-thumbnail-grid edge size.
//
// ponytail: ImageMaxEdge's "unrestricted" (0) doesn't fit a composited grid of downscaled
// frames, so this reuses the tmux-sixel edge as a fixed grid size instead of adding a third
// constant. Every video-thumb cache reader/writer (live request, warm check, prefetch) must
// call this rather than ImageMaxEdge directly, or cache keys stop matching.
func VideoThumbMaxEdge(cfg config.PreviewConfig) int {
	if edge := ImageMaxEdge(cfg); edge >= 1 {
		return edge
	}
	return config.DefaultPreviewTmuxSixelMaxEdgePx
}

// EffectiveStillMaxEdge picks the longest-edge clamp that applies to a still-image decode for
// the given protocol/tmux context: the tmux-sixel payload-safety clamp for Sixel under tmux
// (the one case where a single, unchunked escape sequence can exceed tmux's input buffer), and
// the general clamp (default unrestricted) everywhere else.
func EffectiveStillMaxEdge(cfg config.PreviewConfig, protocol previewpanel.ImageProtocol, inTmux bool) int {
	if protocol == previewpanel.ImageProtocolSixel && inTmux {
		return TmuxSixelMaxEdge(cfg)
	}
	return ImageMaxEdge(cfg)
}
