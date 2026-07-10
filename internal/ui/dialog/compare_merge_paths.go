package dialog

import "github.com/paranoidi/paras-commander/internal/primitive"

const compareMergePathHereLabel = "(here)"

// FormatCompareMergePaths formats compare merge roots for the destination section:
// home-tilde display, optional shared prefix, and per-side suffix labels with
// common leading segments stripped.
func FormatCompareMergePaths(primaryPath, secondaryPath, homeDir string) (sharedPrefix, leftLabel, rightLabel string) {
	leftT := primitive.PathWithHomeTilde(primaryPath, homeDir)
	rightT := primitive.PathWithHomeTilde(secondaryPath, homeDir)
	leftS, rightS := primitive.StripCommonPathPrefix(leftT, rightT)
	if leftS == leftT && rightS == rightT {
		return "", leftT, rightT
	}
	sharedPrefix = primitive.CommonDisplayPathPrefix(leftT, rightT)
	if leftS == "" && rightS == "" {
		return "", leftT, rightT
	}
	leftLabel = leftS
	if leftLabel == "" {
		leftLabel = compareMergePathHereLabel
	}
	rightLabel = rightS
	if rightLabel == "" {
		rightLabel = compareMergePathHereLabel
	}
	return sharedPrefix, leftLabel, rightLabel
}
