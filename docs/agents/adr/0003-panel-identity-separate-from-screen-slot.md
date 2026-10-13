# Panels are identified as Primary/Secondary, not left/right

Panel identity (`ui.PrimaryPanel` / `ui.SecondaryPanel`) is fixed; swap-panes and stacked/side-by-side layout only change which paint slot each identity gets. Swapping therefore never moves panel state. The Left/Right pulldown labels and `TopPanelLeft`/`TopPanelRight` ids remain as visual-slot names.
