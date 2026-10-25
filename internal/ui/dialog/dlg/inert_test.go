package dlg

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func inertSpec() Spec {
	return Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, Unclamped: true,
		Groups: []Group{{Columns: [][]Item{{
			{ID: "a", Kind: KindCheckbox, Label: "Alpha", Mnemonic: 'a'},
			{ID: "b", Kind: KindCheckbox, Label: "Beta", Mnemonic: 'b', Inert: true},
			{ID: "c", Kind: KindCheckbox, Label: "Gamma", Mnemonic: 'c'},
		}}}},
		Buttons: []Button{{ID: OK, Label: "OK", Mnemonic: 'O'}},
	}
}

// An inert item is drawn greyed out but stays a focus stop, mnemonic target and hit target; the
// caller is the one that ignores its activation.
func TestInertItemStaysFocusable(t *testing.T) {
	spec := inertSpec()
	focus := ID("a")
	HandleKey(spec, &focus, key(tcell.KeyDown))
	if focus != "b" {
		t.Fatalf("Down onto an inert item = %q, want b", focus)
	}
	HandleKey(spec, &focus, key(tcell.KeyDown))
	if focus != "c" {
		t.Fatalf("Down past an inert item = %q, want c", focus)
	}
	HandleKey(spec, &focus, key(tcell.KeyUp))
	if ev, ok := HandleKey(spec, &focus, runeKey(' ', tcell.ModNone)); !ok || ev.ID != "b" {
		t.Errorf("Space on an inert item = (%q, %v), want an event for b", ev.ID, ok)
	}
	focus = "a"
	if ev, ok := HandleKey(spec, &focus, runeKey('b', tcell.ModAlt)); !ok || ev.ID != "b" || focus != "b" {
		t.Errorf("Alt+b = (%q, %v) focus=%q, want an event for b and focus on it", ev.ID, ok, focus)
	}
	p := mustPlace(t, spec)
	b, _ := p.Item("b")
	if b.Disabled {
		t.Error("an inert item is not a disabled placement")
	}
	if id, hit := HitTest(p, b.Rect.X+2, b.Rect.Y); !hit || id != "b" {
		t.Errorf("click on an inert item = (%q, %v), want a hit on b", id, hit)
	}
}

func TestInertItemDrawsDisabled(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	spec := inertSpec()
	Draw(screen, 80, 24, spec, "a", styles)
	screen.Show()
	b, _ := mustPlace(t, spec).Item("b")
	_, st, _ := screen.Get(b.Rect.X+6, b.Rect.Y)
	if want := styles.DialogOptionDisabledStyle(); st != want {
		t.Errorf("inert checkbox style = %v, want the disabled style %v", st, want)
	}
}
