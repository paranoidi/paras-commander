package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestHashScreenLogicalStableForEmptyScreen(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(10, 5)
	screen.Clear()
	a := HashScreenLogical(screen)
	b := HashScreenLogical(screen)
	if a != b {
		t.Fatalf("hash = %d second = %d", a, b)
	}
}

func TestHashScreenLogicalChangesWhenCellChanges(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(10, 5)
	screen.Clear()
	before := HashScreenLogical(screen)
	screen.SetContent(3, 2, 'x', nil, tcell.StyleDefault)
	after := HashScreenLogical(screen)
	if after == before {
		t.Fatal("expected hash to change after SetContent")
	}
}
