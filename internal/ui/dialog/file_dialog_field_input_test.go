package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/pathpick"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

func TestDrawInputFieldScrollsHorizontallyForLongValue(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(40, 5)

	styles := theme.Default()
	value := "/home/user/very/long/path/to/something"
	const width = 20
	field := FileDialogField{Value: value, Cursor: len([]rune(value))}

	drawInputField(screen, 1, 1, width, field, true, styles)

	got := tcelltest.TextAt(screen, 1, 1, width)
	if strings.Contains(got, "~") {
		t.Fatalf("did not expect ~ truncation marker, got %q", got)
	}
	tail := []rune(value)[len([]rune(value))-(width-2):]
	if !strings.Contains(got, string(tail)) {
		t.Fatalf("expected tail %q in visible row %q", string(tail), got)
	}
	left := string(draw.ScrollOverflowLeft)
	if !strings.Contains(got, left) {
		t.Fatalf("expected %s overflow marker in %q", left, got)
	}
}

func TestDrawInputFieldPathPickerInvalidDoesNotRenderCompletion(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(40, 5)

	styles := theme.Default()

	value := "/tmp/x"
	cursor := len([]rune(value))
	const width = 22
	field := FileDialogField{
		Value:      value,
		Cursor:     cursor,
		PathPicker: true,
	}
	// An open completion dropdown must not leak a suggestion into the input row: only the
	// typed value is painted there, invalid or not (the dropdown itself shows suggestions).
	// Two candidates (not the single-prefix ghost case) keeps the dropdown open instead of
	// showing ghost text in the row.
	field.Completion.Set(value, cursor-1, []pathpick.Candidate{{Name: "xYZ"}, {Name: "xAB"}})

	drawInputFieldInvalid(screen, 1, 1, width, field, true, true, styles)

	got := tcelltest.TextAt(screen, 1, 1, width)
	if strings.Contains(got, "Z") {
		t.Fatalf("input row must render only the typed value, got %q", got)
	}
	if !strings.HasPrefix(got, value) {
		t.Fatalf("input row = %q, want it to start with typed value %q", got, value)
	}
}

func TestDrawInputFieldShowsGhostSuffixForSinglePrefixCandidateNoDropdown(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(40, 5)

	styles := theme.Default()
	value := "/tmp/lant"
	cursor := len([]rune(value))
	const width = 30
	field := FileDialogField{Value: value, Cursor: cursor, PathPicker: true}
	field.Completion.Set(value, 5, []pathpick.Candidate{{Name: "lantern", IsDir: true}})
	if field.Completion.Open {
		t.Fatal("a single prefix candidate must not open the dropdown")
	}

	drawInputField(screen, 1, 1, width, field, true, styles)

	got := tcelltest.TextAt(screen, 1, 1, width)
	const suffix = "ern"
	if !strings.Contains(got, suffix) {
		t.Fatalf("expected ghost suffix %q in row %q", suffix, got)
	}

	_, wantPhFG, _ := styles.DialogInputActivePlaceholder.Decompose()
	ghostCol := 1 + cursor + len(suffix) - 1
	_, gotSt, _ := screen.Get(ghostCol, 1)
	gotFG, _, _ := gotSt.Decompose()
	if gotFG != wantPhFG {
		t.Fatalf("ghost cell fg = %v, want placeholder fg %v", gotFG, wantPhFG)
	}

	// The dropdown itself is a no-op while only ghost text is showing (Open is false).
	before := tcelltest.TextAt(screen, 1, 2, width)
	drawPathCompletionDropdown(screen, 1, 2, 0, field.Completion, uiscrollbar.StyleThumb, styles)
	after := tcelltest.TextAt(screen, 1, 2, width)
	if before != after {
		t.Fatalf("drawPathCompletionDropdown painted something while Open=false: before %q after %q", before, after)
	}
}

func TestDrawInputFieldPathPickerScrollsHorizontallyForLongValue(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(40, 5)

	styles := theme.Default()
	value := "/home/user/very/long/path/to/something"
	const width = 20
	field := FileDialogField{
		Value:      value,
		Cursor:     len([]rune(value)),
		Scroll:     0,
		PathPicker: true,
	}

	drawInputField(screen, 1, 1, width, field, true, styles)

	got := tcelltest.TextAt(screen, 1, 1, width)
	if strings.Contains(got, "~") {
		t.Fatalf("did not expect ~ truncation marker, got %q", got)
	}
	left := string(draw.ScrollOverflowLeft)
	if !strings.Contains(got, left) {
		t.Fatalf("expected %s overflow marker in %q", left, got)
	}
}
