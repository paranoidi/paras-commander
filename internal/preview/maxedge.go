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

// VideoThumbMaxEdge returns the configured video-thumb-grid edge size for protocols/contexts
// that don't need the tmux-sixel payload-safety clamp, or the built-in default. Unlike
// ImageMaxEdge, 0/unset always falls back to a concrete default: a video-thumb grid composites
// native-resolution frames before this clamp applies, so it can't go unrestricted.
func VideoThumbMaxEdge(cfg config.PreviewConfig) int {
	if cfg.VideoThumbMaxEdgePx < 1 {
		return config.DefaultPreviewVideoThumbMaxEdgePx
	}
	return cfg.VideoThumbMaxEdgePx
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

// EffectiveVideoThumbMaxEdge picks the longest-edge clamp that applies to a video-thumb grid
// composite for the given protocol/tmux context, mirroring EffectiveStillMaxEdge: an explicit
// ImageMaxEdgePx override always wins (matching video's pre-existing behavior), then the
// tmux-sixel payload-safety clamp for Sixel under tmux, then the higher (but still bounded)
// video-thumb default everywhere else.
func EffectiveVideoThumbMaxEdge(cfg config.PreviewConfig, protocol previewpanel.ImageProtocol, inTmux bool) int {
	if edge := ImageMaxEdge(cfg); edge >= 1 {
		return edge
	}
	if protocol == previewpanel.ImageProtocolSixel && inTmux {
		return TmuxSixelMaxEdge(cfg)
	}
	return VideoThumbMaxEdge(cfg)
}
