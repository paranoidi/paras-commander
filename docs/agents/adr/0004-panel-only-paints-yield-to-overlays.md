# Panel-only partial repaints decline while a dialog or menu is open

The cheap panel-only paint paths (list-nav updates, async listing/carousel results, disk-usage updates) check `App.panelOnlyPaintBlocked()` and fall back to a full `ui.Render` whenever a modal dialog or menu is open. Without the gate an async listing landing under a dialog painted the file list over it. Cost: losing the cheap repaint exactly when async results arrive under an overlay.
