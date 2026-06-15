package app

import (
	"errors"
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

func (a *App) closeCarouselFilePreview() {
	a.commandsMu.Lock()
	a.model.CarouselFilePreview = ui.FilePreviewState{}
	a.commandsMu.Unlock()
	a.carouselFilePreviewRunGen.Add(1)
	a.carouselFilePreviewLastFingerprint = ""
}

func (a *App) patchCarouselFilePreview(fn func(*ui.FilePreviewState)) {
	a.patchPreviewState(previewTargetCarousel, fn)
}

func (a *App) patchCarouselFilePreviewMessage(titleBase, msg string) {
	a.patchCarouselFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.Path = ""
		st.TitleBase = titleBase
		st.CombinedText = ""
		st.Scroll = 0
		st.ExitCode = 0
		st.ErrorMsg = msg
	})
	a.postCommandWake()
}

func (a *App) activePanelFileColumnRect() (geom.Rect, bool) {
	w, h := a.screen.Size()
	lay := a.layoutForTerminalSize(w, h)
	if lay.TooSmall {
		return geom.Rect{}, false
	}
	col := lay.Left
	if a.model.ActivePanel == ui.RightPanel {
		col = lay.Right
	}
	stripN := ui.SelectionsStripLayoutItemCount(
		a.activePanel(),
		a.model.ActivePanel,
		a.model.ActivePanel,
		a.model.ThemeDialog.Open,
	)
	fileCol, _ := ui.SplitPanelColumn(ui.Rect(col), stripN, a.model.SelectionsPanelMaxRows, ui.MinFileListContentRows)
	if fileCol.Width <= 0 {
		return geom.Rect{}, false
	}
	return geom.Rect(fileCol), true
}

func (a *App) carouselFilePreviewEligible() bool {
	rect, ok := a.activePanelFileColumnRect()
	if !ok {
		return false
	}
	return panelcarousel.FilePreviewEligible(rect, a.model.HideInactivePanel)
}

func (a *App) carouselChildPreviewLayoutMetrics() (textW, contentH int, ok bool) {
	rect, ok := a.activePanelFileColumnRect()
	if !ok {
		return 1, 0, false
	}
	childW := panelcarousel.ChildColumnWidth(rect)
	tw := childW - 2
	if tw < 1 {
		tw = 1
	}
	listH := geom.PanelListRows(rect)
	if listH < 1 {
		return tw, 0, false
	}
	return tw, listH, true
}

func (a *App) carouselFilePreviewWantPath() (path string, ok bool) {
	p := a.activePanel()
	entry, okEntry := p.CurrentEntry()
	if !okEntry || entry.Type == localfs.EntryDirectory {
		return "", false
	}
	path = filepath.Clean(entry.Path)
	if path == "" || path == "." {
		return "", false
	}
	return path, true
}

func (a *App) carouselFilePreviewFingerprint() string {
	path, ok := a.carouselFilePreviewWantPath()
	if !ok {
		return "none"
	}
	return "f:" + path
}

func (a *App) carouselFilePreviewContext() bool {
	if a.model.ViewMode != ui.ViewBrowser {
		return false
	}
	p := a.activePanel()
	if !p.CarouselMode {
		return false
	}
	if a.model.QuickViewDisplayActive() {
		return false
	}
	if !a.carouselFilePreviewEligible() {
		return false
	}
	rect, ok := a.activePanelFileColumnRect()
	if !ok || !panelcarousel.LayoutFits(rect, 0, 0) {
		return false
	}
	if !panelcarousel.ShowChildPreviewColumn(*p, false, true) {
		return false
	}
	if panelcarousel.ChildPreviewKindFor(*p, false, true) != panelcarousel.ChildPreviewFile {
		return false
	}
	if a.model.Menu.Open || a.model.ModalDialogOpen() || a.inQuickFilterUI() {
		return false
	}
	if a.model.ActiveSubFocus != ui.SubFocusFileList {
		return false
	}
	if a.model.ActivePanel != ui.LeftPanel && a.model.ActivePanel != ui.RightPanel {
		return false
	}
	if _, ok := a.carouselFilePreviewWantPath(); !ok {
		return false
	}
	return true
}

func (a *App) applyCarouselFilePreviewNow() {
	if !a.carouselFilePreviewContext() {
		a.closeCarouselFilePreview()
		return
	}
	path, ok := a.carouselFilePreviewWantPath()
	if !ok {
		a.closeCarouselFilePreview()
		return
	}
	workDir := a.activePanel().PathString()
	if err := localfs.CheckFilePreviewable(path); err != nil {
		switch {
		case errors.Is(err, localfs.ErrFilePreviewBinary):
			a.patchCarouselFilePreviewMessage(filepath.Base(path), "Not a text file")
		case errors.Is(err, localfs.ErrFilePreviewIsDir):
			a.patchCarouselFilePreviewMessage("", "Not a file")
		default:
			a.patchCarouselFilePreviewMessage(filepath.Base(path), err.Error())
		}
		return
	}
	tw, _, layOK := a.carouselChildPreviewLayoutMetrics()
	if !layOK {
		tw = 1
	}
	argv, err := cmdrun.BuildFilePreviewArgv(a.config.Preview.Command, path, tw)
	if err != nil {
		a.patchCarouselFilePreviewMessage(filepath.Base(path), "Preview command: "+err.Error())
		return
	}
	titleBase := filepath.Base(path)
	a.patchCarouselFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhasePending
		st.Path = path
		st.TitleBase = titleBase
		st.CombinedText = ""
		st.Scroll = 0
		st.ExitCode = 0
		st.ErrorMsg = ""
	})
	a.postCommandWake()
	gen := a.carouselFilePreviewRunGen.Add(1)
	go a.runFilePreview(a.commandsCtx, path, argv, workDir, previewTargetCarousel, gen)
}

func (a *App) reconcileCarouselFilePreview() {
	p := a.activePanel()
	eligible := a.carouselFilePreviewEligible()
	kind := panelcarousel.ChildPreviewKindFor(*p, a.model.QuickViewDisplayActive(), eligible)
	if kind != panelcarousel.ChildPreviewFile {
		a.commandsMu.RLock()
		open := a.model.CarouselFilePreview.Open
		a.commandsMu.RUnlock()
		if open || a.carouselFilePreviewLastFingerprint != "" {
			a.closeCarouselFilePreview()
		}
		return
	}
	if a.model.Menu.Open || a.model.ModalDialogOpen() || a.inQuickFilterUI() {
		return
	}
	if a.model.ActiveSubFocus != ui.SubFocusFileList {
		return
	}
	if a.carouselPreviewNavSkipSnapshot.Load() {
		return
	}
	sig := a.carouselFilePreviewFingerprint()
	if sig == a.carouselFilePreviewLastFingerprint {
		return
	}
	if a.config.UI.CarouselPreviewDebounceMS <= 0 {
		a.applyCarouselFilePreviewAfterFlush()
		return
	}
	a.armCarouselPreviewNavCoalesceAfterListNav()
}

func (a *App) applyCarouselFilePreviewAfterFlush() {
	if !a.carouselFilePreviewContext() {
		a.closeCarouselFilePreview()
		return
	}
	a.applyCarouselFilePreviewNow()
	a.carouselFilePreviewLastFingerprint = a.carouselFilePreviewFingerprint()
}
