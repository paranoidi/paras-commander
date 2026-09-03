// Command timingcheck exercises the real select-all code path (panel.State.Load,
// panel.State.SelectGroup, and the disk-usage "needs scan" filter) against a live
// directory, to measure where wall-clock time actually goes on slow storage.
// Point it at ./slowfs/mount to validate the select-all UI-freeze fix under injected
// FUSE latency instead of eyeballing the interactive TUI.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func main() {
	dir := flag.String("dir", "", "directory to test (e.g. ./slowfs/mount)")
	flag.Parse()
	if *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: timingcheck --dir=./slowfs/mount")
		os.Exit(2)
	}

	var s panel.State
	t0 := time.Now()
	if err := s.Load(*dir); err != nil {
		fmt.Fprintf(os.Stderr, "Load: %v\n", err)
		os.Exit(1)
	}
	loadDur := time.Since(t0)
	fmt.Printf("Load(%d entries):        %v\n", len(s.Entries), loadDur)

	t0 = time.Now()
	if _, err := s.SelectGroup("*", false, false, false, panel.GroupPatternShell, panel.GroupSelectMeta{}); err != nil {
		fmt.Fprintf(os.Stderr, "SelectGroup: %v\n", err)
		os.Exit(1)
	}
	selectDur := time.Since(t0)
	fmt.Printf("SelectGroup(\"*\") [Fix 1]: %v  (%d selected)\n", selectDur, s.SelectedPathCount())

	engine := diskusage.New()

	// SelectionSizeLabel is what the render loop calls on EVERY frame to draw the footer
	// ("N items (X bytes)") whenever a selection exists (internal/ui/panel_render.go). Before
	// Fix 3 it fell back to a live Stat (DiskScanExcluded) per selected directory not yet
	// cached — i.e. this ran the full synchronous cost on every single render, not just once,
	// for as long as the background scan hadn't finished. Timing repeated calls here (with a
	// freshly-created engine that has nothing cached yet, worst case) shows whether that's
	// still true.
	const renders = 5
	var labelDur time.Duration
	for i := 0; i < renders; i++ {
		t0 = time.Now()
		label, _ := ui.SelectionSizeLabel(&s, false, engine, "*")
		labelDur += time.Since(t0)
		if i == 0 {
			fmt.Printf("SelectionSizeLabel first call:          %v  (%q)\n", time.Since(t0), label)
		}
	}
	fmt.Printf("SelectionSizeLabel [Fix 3] %d renders, nothing cached yet: total %v, avg %v\n",
		renders, labelDur, labelDur/renders)

	// This is the per-selected-directory mount-exclusion stat check that reconcileSelectionSizeScans
	// now runs on a debounced BACKGROUND goroutine (Fix 2) instead of inline on the main goroutine.
	// Timing it directly here shows what the main goroutine used to pay synchronously.
	roots := append([]string(nil), s.PrunedSelectionRoots()...)
	byPath := s.EntriesByPath()
	t0 = time.Now()
	need := diskusage.DirectoriesNeedingScan(roots, byPath, s.ListingDevice, s.ListingDeviceValid, engine, false, nil)
	scanFilterDur := time.Since(t0)
	fmt.Printf("DirectoriesNeedingScan [Fix 2 background-only cost]: %v  (%d dirs need scanning)\n", scanFilterDur, len(need))

	// Now that the background pass has run (populating the engine's excluded-cache for any
	// mount-excluded dirs, though none here), confirm SelectionSizeLabel is still cheap.
	t0 = time.Now()
	ui.SelectionSizeLabel(&s, false, engine, "*")
	fmt.Printf("SelectionSizeLabel after background pass: %v\n", time.Since(t0))

	fmt.Println()
	fmt.Println("With the fix: Load pays the unavoidable directory-listing cost of slow storage.")
	fmt.Println("SelectGroup, SelectionSizeLabel, and DirectoriesNeedingScan are what select-all")
	fmt.Println("itself used to block on. SelectGroup and SelectionSizeLabel should stay small")
	fmt.Println("regardless of disk speed and render count (pure in-memory / cache-only now);")
	fmt.Println("DirectoriesNeedingScan's cost (proportional to selection size * per-stat latency)")
	fmt.Println("now runs off the main goroutine instead of blocking input handling.")
}
