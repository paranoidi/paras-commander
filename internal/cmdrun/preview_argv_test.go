package cmdrun

import (
	"slices"
	"testing"
)

func TestPreviewCommandArgvAppendsPath(t *testing.T) {
	got, err := PreviewCommandArgv("bat --foo", "/tmp/a b.txt", 80)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"bat", "--foo", "/tmp/a b.txt"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestPreviewCommandArgvPlaceholder(t *testing.T) {
	got, err := PreviewCommandArgv(`bat --paging=never %f --color=always`, "/tmp/x.go", 80)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"bat", "--paging=never", "/tmp/x.go", "--color=always"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestPreviewCommandArgvTerminalWidthPlaceholder(t *testing.T) {
	got, err := PreviewCommandArgv(`bat --terminal-width=%w --wrap=auto`, "/tmp/x.go", 42)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"bat", "--terminal-width=42", "--wrap=auto", "/tmp/x.go"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestPreviewCommandArgvTerminalWidthClampedLow(t *testing.T) {
	got, err := PreviewCommandArgv(`bat --tw=%w`, "/a", 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"bat", "--tw=1", "/a"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestPreviewCommandArgvMultiplePlaceholderErrors(t *testing.T) {
	_, err := PreviewCommandArgv(`bat %f %f`, "/x", 80)
	if err == nil {
		t.Fatal("want error")
	}
}

func TestPreviewCommandArgvRejectsLegacyPlaceholders(t *testing.T) {
	_, err := PreviewCommandArgv(`bat {path}`, "/x", 80)
	if err == nil {
		t.Fatal("want error for {path}")
	}
}
