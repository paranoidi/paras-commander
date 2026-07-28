package preview

import (
	"github.com/paranoidi/paras-commander/internal/config"
)

// ImageMaxEdge returns the configured longest-edge clamp, or the built-in default.
func ImageMaxEdge(cfg config.PreviewConfig) int {
	if cfg.ImageMaxEdgePx < 1 {
		return config.DefaultPreviewImageMaxEdgePx
	}
	return cfg.ImageMaxEdgePx
}
