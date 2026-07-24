package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/textutil"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestSetErrorMessageOpsErrorNoDuplicatePrefix(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	err := &ops.Error{Op: "mkdir", Text: "target already exists"}
	app.setErrorMessage("Mkdir", err)
	if app.model.Message != "mkdir: target already exists" {
		t.Fatalf("message = %q, want mkdir: target already exists", app.model.Message)
	}
}

func TestSetErrorMessageExecuteFailureNoDuplicatePrefix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		prefix string
		err    error
		want   string
	}{
		{
			prefix: "Mkdir failed",
			err:    fmt.Errorf(`mkdir "/tmp/x": file exists`),
			want:   `mkdir "/tmp/x": file exists`,
		},
		{
			prefix: "Rename failed",
			err:    fmt.Errorf(`rename "/a" to "/b": permission denied`),
			want:   `rename "/a" to "/b": permission denied`,
		},
		{
			prefix: "Hardlink failed",
			err:    fmt.Errorf(`link "/dst" -> "/src": file exists`),
			want:   `link "/dst" -> "/src": file exists`,
		},
		{
			prefix: "Mass rename failed",
			err:    fmt.Errorf(`mass rename stage1 "/a": permission denied`),
			want:   `mass rename stage1 "/a": permission denied`,
		},
		{
			prefix: "Chmod failed",
			err:    &ops.Error{Op: "chmod", Text: "failed to change mode for foo", Err: errors.New(`chmod "/p": permission denied`)},
			want:   `chmod: failed to change mode for foo (chmod "/p": permission denied)`,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.prefix, func(t *testing.T) {
			t.Parallel()
			app := testAppMinimal(t)
			app.setErrorMessage(tc.prefix, tc.err)
			if app.model.Message != tc.want {
				t.Fatalf("message = %q, want %q", app.model.Message, tc.want)
			}
		})
	}
}

func TestSetErrorMessageKeepsUnrelatedPrefix(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	err := fmt.Errorf(`read directory "/tmp": permission denied`)
	app.setErrorMessage("Enter failed", err)
	want := `Enter failed: read directory "/tmp": permission denied`
	if app.model.Message != want {
		t.Fatalf("message = %q, want %q", app.model.Message, want)
	}
}

func TestShouldOmitErrorPrefix(t *testing.T) {
	t.Parallel()
	if !shouldOmitErrorPrefix("Mkdir failed", fmt.Errorf(`mkdir "/x": exists`)) {
		t.Fatal("expected omit for mkdir execute error")
	}
	if shouldOmitErrorPrefix("Enter failed", fmt.Errorf(`read directory "/x": denied`)) {
		t.Fatal("expected keep prefix for unrelated enter error")
	}
}

func TestTransientErrorTextPermissionDenied(t *testing.T) {
	wrapped := fmt.Errorf(`read directory "/home/nella": %w`, fs.ErrPermission)
	if got := transientErrorText(wrapped); got != "permission denied" {
		t.Fatalf("transientErrorText() = %q, want permission denied", got)
	}
	if got := transientErrorText(errors.New("other")); got != "other" {
		t.Fatalf("transientErrorText() = %q, want literal message when not ErrPermission", got)
	}
}

func TestSetErrorMessageEnterNotExistNoDuplicatePath(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "gone")
	_, err := localfs.ListDir(missing, localfs.DefaultListOptions())
	if err == nil {
		t.Fatal("ListDir() error = nil, want error")
	}
	want := fmt.Sprintf(`Enter failed: no such directory %q`, missing)
	got, ok := enterFailedMessage(err)
	if !ok {
		t.Fatalf("enterFailedMessage() ok = false, want true for %v", err)
	}
	if got != want {
		t.Fatalf("enterFailedMessage() = %q, want %q", got, want)
	}
}

func TestSetErrorMessageEnterNotExistShowsShortMessage(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	err := fmt.Errorf(`stat directory "/tmp/missing": %w`, fs.ErrNotExist)
	want := `Enter failed: no such directory "/tmp/missing"`
	app.setErrorMessage("Enter failed", err)
	if app.model.Message != want {
		t.Fatalf("message = %q, want %q", app.model.Message, want)
	}
}

func TestEnterFailedMessage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  error
		want string
		ok   bool
	}{
		{
			err:  fmt.Errorf(`stat directory "/tmp/missing": %w`, fs.ErrNotExist),
			want: `Enter failed: no such directory "/tmp/missing"`,
			ok:   true,
		},
		{
			err:  &os.PathError{Op: "stat", Path: "/tmp/x", Err: fs.ErrNotExist},
			want: `Enter failed: no such directory "/tmp/x"`,
			ok:   true,
		},
		{
			err: fmt.Errorf(`read directory "/tmp": %w`, fs.ErrPermission),
			ok:  false,
		},
	}
	for _, tc := range cases {
		got, ok := enterFailedMessage(tc.err)
		if ok != tc.ok {
			t.Fatalf("enterFailedMessage(%v) ok = %v, want %v", tc.err, ok, tc.ok)
		}
		if got != tc.want {
			t.Fatalf("enterFailedMessage(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}

func TestJobFailureBannerDetail(t *testing.T) {
	t.Parallel()
	wrapped := fmt.Errorf(`create directory "/very/long/path/nested": %w`, fs.ErrPermission)
	if got := jobFailureBannerDetail(wrapped, wrapped.Error()); got != "permission denied" {
		t.Fatalf("jobFailureBannerDetail(wrapped) = %q, want permission denied", got)
	}
	if got := jobFailureBannerDetail(nil, `open "/tmp/x": permission denied`); got != "permission denied" {
		t.Fatalf("jobFailureBannerDetail(nil, ...) = %q, want permission denied", got)
	}
	long := strings.Repeat("a", 120)
	got := jobFailureBannerDetail(errors.New(long), "")
	if got == "" || strings.TrimSuffix(got, "…") == got {
		t.Fatalf("expected truncated banner with ellipsis, got %q", got)
	}
	if utf8.RuneCountInString(got) != textutil.BannerMaxRunes+1 {
		t.Fatalf("len runes = %d, want %d", utf8.RuneCountInString(got), textutil.BannerMaxRunes+1)
	}
}

func TestJobFailureLogDetailKeepsFullError(t *testing.T) {
	t.Parallel()
	err := &ops.Error{
		Op:   "delete",
		Text: "failed to delete stale-node",
		Err:  errors.New("directory not empty"),
	}
	errText := err.Error()
	if got := jobFailureLogDetail(err, errText); got != errText {
		t.Fatalf("jobFailureLogDetail() = %q, want %q", got, errText)
	}
}

func TestMessageLogNewestFirst(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	app.setTransientMessage("first", ui.MessageUrgencyInfo)
	app.setTransientMessage("second", ui.MessageUrgencyInfo)
	if len(app.model.MessageLog) < 2 {
		t.Fatalf("log len = %d, want at least 2", len(app.model.MessageLog))
	}
	if app.model.MessageLog[0].Text != "second" {
		t.Fatalf("log[0] = %q, want newest message first", app.model.MessageLog[0].Text)
	}
	if app.model.MessageLog[len(app.model.MessageLog)-1].Text != "first" {
		t.Fatalf("log[last] = %q, want oldest message last", app.model.MessageLog[len(app.model.MessageLog)-1].Text)
	}
}

func TestClearMessageLog(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	app.setTransientMessage("hello", ui.MessageUrgencyInfo)
	app.clearMessageLog()
	if len(app.model.MessageLog) != 0 {
		t.Fatalf("MessageLog len = %d, want 0", len(app.model.MessageLog))
	}
	if app.model.Message != "" {
		t.Fatalf("banner = %q, want empty", app.model.Message)
	}
}

func TestSetJobFailedTransientMessageLogsFullError(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 120, 24)
	app := newApp(t, screen, dir)

	err := &ops.Error{
		Op:   "delete",
		Text: "failed to delete stale-node",
		Err:  errors.New("directory not empty"),
	}
	errText := err.Error()
	app.setTransientMessageBanner(
		fmt.Sprintf("Job failed: %s", jobFailureLogDetail(err, errText)),
		fmt.Sprintf("Job failed: %s", jobFailureBannerDetail(err, errText)),
		ui.MessageUrgencyError,
	)
	if len(app.model.MessageLog) == 0 {
		t.Fatal("expected message log entry")
	}
	full := strings.Join(func() []string {
		var parts []string
		for _, e := range app.model.MessageLog {
			parts = append(parts, e.Text)
		}
		return parts
	}(), " ")
	if !strings.Contains(full, "directory not empty") {
		t.Fatalf("log = %q, want full error including reason", full)
	}
	if strings.Contains(app.model.Message, "remove \"") {
		t.Fatalf("banner should omit repeated remove paths, got %q", app.model.Message)
	}
	if utf8.RuneCountInString(app.model.Message) > textutil.BannerMaxRunes+len("Job failed: ") {
		t.Fatalf("banner too long, got %q", app.model.Message)
	}
}
