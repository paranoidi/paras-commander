package keymap

import (
	"fmt"
	"sort"
	"strings"
)

// OverlaySpec describes one keymap overlay table (e.g. [jobs]).
type OverlaySpec struct {
	TableName string
	Defaults  func() map[string][]string
	Allowed   func(actionID string) bool
}

// overlayRegistry is the single source of overlay table metadata and ordering.
// Order matches Bundle overlay field assignment in buildBundle.
var overlayRegistry = []OverlaySpec{
	{TableName: JobsShortcutsTable, Defaults: DefaultJobsOverlayKeys, Allowed: AllowedInJobsOverlay},
	{TableName: CommandsShortcutsTable, Defaults: DefaultCommandsOverlayKeys, Allowed: AllowedInCommandsOverlay},
	{TableName: MessagesShortcutsTable, Defaults: DefaultMessagesOverlayKeys, Allowed: AllowedInMessagesOverlay},
	{TableName: FilePreviewShortcutsTable, Defaults: DefaultFilePreviewOverlayKeys, Allowed: AllowedInFilePreviewOverlay},
	{TableName: DialogInputShortcutsTable, Defaults: DefaultDialogInputOverlayKeys, Allowed: AllowedInDialogInputOverlay},
	{TableName: DialogRenameShortcutsTable, Defaults: DefaultRenameDialogOverlayKeys, Allowed: AllowedInRenameDialogOverlay},
	{TableName: DialogMkdirShortcutsTable, Defaults: DefaultMkdirDialogOverlayKeys, Allowed: AllowedInMkdirDialogOverlay},
	{TableName: DialogBookmarkShortcutsTable, Defaults: DefaultBookmarkDialogOverlayKeys, Allowed: AllowedInBookmarkDialogOverlay},
	{TableName: DialogFindShortcutsTable, Defaults: DefaultFindDialogOverlayKeys, Allowed: AllowedInFindDialogOverlay},
	{TableName: DialogHistoryShortcutsTable, Defaults: DefaultHistoryDialogOverlayKeys, Allowed: AllowedInHistoryDialogOverlay},
	{TableName: DialogFlattenShortcutsTable, Defaults: DefaultFlattenDialogOverlayKeys, Allowed: AllowedInFlattenDialogOverlay},
	{TableName: DialogTransferShortcutsTable, Defaults: DefaultTransferDialogOverlayKeys, Allowed: AllowedInTransferDialogOverlay},
	{TableName: CompareShortcutsTable, Defaults: DefaultCompareOverlayKeys, Allowed: AllowedInCompareOverlay},
	{TableName: DedupShortcutsTable, Defaults: DefaultDedupOverlayKeys, Allowed: AllowedInDedupOverlay},
	{TableName: TerminalShortcutsTable, Defaults: DefaultTerminalOverlayKeys, Allowed: AllowedInTerminalOverlay},
	{TableName: DialogMassRenameShortcutsTable, Defaults: DefaultMassRenameDialogOverlayKeys, Allowed: AllowedInMassRenameDialogOverlay},
	{TableName: DialogRunForEachShortcutsTable, Defaults: DefaultRunForEachDialogOverlayKeys, Allowed: AllowedInRunForEachDialogOverlay},
	{TableName: DialogPinShortcutsTable, Defaults: DefaultPinDialogOverlayKeys, Allowed: AllowedInPinDialogOverlay},
	{TableName: DialogHelpShortcutsTable, Defaults: DefaultHelpDialogOverlayKeys, Allowed: AllowedInHelpDialogOverlay},
	{TableName: DialogUserMenuShortcutsTable, Defaults: DefaultUserMenuOverlayKeys, Allowed: AllowedInUserMenuOverlay},
}

// OverlayTableNames returns all overlay TOML table names in registry order.
func OverlayTableNames() []string {
	names := make([]string, len(overlayRegistry))
	for i, spec := range overlayRegistry {
		names[i] = spec.TableName
	}
	return names
}

func validateOverlayKeysFromFile(keys map[string][]string, label string, spec OverlaySpec) error {
	if keys == nil {
		return nil
	}
	for action, chords := range keys {
		if len(chords) == 0 {
			return fmt.Errorf("parse keybindings %q: [%s] action %q has empty key list", label, spec.TableName, action)
		}
		if !spec.Allowed(action) {
			return fmt.Errorf("parse keybindings %q: [%s] action %q is not allowed (%s)", label, spec.TableName, action, overlayNotAllowedHint(spec))
		}
	}
	return nil
}

func overlayAllowedActionIDs(spec OverlaySpec) []string {
	if spec.Allowed == nil {
		return nil
	}
	ids := make([]string, 0)
	for id := range KnownActions {
		if spec.Allowed(id) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func overlayNotAllowedHint(spec OverlaySpec) string {
	ids := overlayAllowedActionIDs(spec)
	if len(ids) == 0 {
		return "not allowed"
	}
	return strings.Join(ids, ", ") + " only"
}

func overlayStubHeaderComments() string {
	var b strings.Builder
	for _, spec := range overlayRegistry {
		ids := overlayAllowedActionIDs(spec)
		if len(ids) == 0 {
			continue
		}
		b.WriteString("# [")
		b.WriteString(spec.TableName)
		b.WriteString("] — ")
		b.WriteString(strings.Join(ids, ", "))
		b.WriteString(".\n")
	}
	return b.String()
}

func defaultOverlayLayers() []map[string][]string {
	layers := make([]map[string][]string, len(overlayRegistry))
	for i, spec := range overlayRegistry {
		layers[i] = spec.Defaults()
	}
	return layers
}
