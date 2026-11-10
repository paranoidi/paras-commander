package ops

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/archive"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

const memRemoteHost = "sftp://alice@example.com"

// memRemote is an in-memory remote backend: files and directories keyed by their location string.
type memRemote struct {
	mu    sync.Mutex
	files map[string][]byte
	dirs  map[string]bool
	links map[string]string // link location -> target location
}

func newMemRemote(dirs ...string) *memRemote {
	m := &memRemote{files: map[string][]byte{}, dirs: map[string]bool{memRemoteHost: true}, links: map[string]string{}}
	for _, d := range dirs {
		m.dirs[memRemoteHost+d] = true
	}
	return m
}

func (m *memRemote) Scheme() pathloc.Scheme { return pathloc.SchemeSFTP }

func (m *memRemote) entry(loc pathloc.Path) (fsbackend.Entry, bool) {
	k := loc.String()
	if _, ok := m.links[k]; ok {
		return fsbackend.Entry{Name: loc.Base(), Loc: loc, Type: fsbackend.EntrySymlink, Size: int64(len(m.links[k])), Mode: fs.ModeSymlink | 0o777}, true
	}
	if m.dirs[k] {
		return fsbackend.Entry{Name: loc.Base(), Loc: loc, Type: fsbackend.EntryDirectory, Mode: 0o755}, true
	}
	if b, ok := m.files[k]; ok {
		return fsbackend.Entry{Name: loc.Base(), Loc: loc, Type: fsbackend.EntryFile, Size: int64(len(b)), Mode: 0o644, ModifiedAt: time.Now()}, true
	}
	return fsbackend.Entry{}, false
}

func (m *memRemote) Stat(_ context.Context, loc pathloc.Path) (fsbackend.Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.entry(loc); ok {
		return e, nil
	}
	return fsbackend.Entry{}, os.ErrNotExist
}

// StatFollow is Stat through a symlink; a dangling link is not found.
func (m *memRemote) StatFollow(ctx context.Context, loc pathloc.Path) (fsbackend.Entry, error) {
	m.mu.Lock()
	target, ok := m.links[loc.String()]
	m.mu.Unlock()
	if !ok {
		return m.Stat(ctx, loc)
	}
	return m.Stat(ctx, pathloc.MustParse(target))
}

func (m *memRemote) List(_ context.Context, dir pathloc.Path) ([]fsbackend.Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []fsbackend.Entry
	for _, set := range []map[string]bool{m.dirs, nil} {
		if set == nil {
			for k := range m.files {
				if loc := pathloc.MustParse(k); loc.Parent().Equal(dir) {
					e, _ := m.entry(loc)
					out = append(out, e)
				}
			}
			continue
		}
		for k := range set {
			if loc := pathloc.MustParse(k); !loc.Equal(dir) && loc.Parent().Equal(dir) {
				e, _ := m.entry(loc)
				out = append(out, e)
			}
		}
	}
	return out, nil
}

type memReader struct{ *bytes.Reader }

func (memReader) Close() error { return nil }

func (m *memRemote) OpenRead(_ context.Context, loc pathloc.Path) (io.ReadSeekCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.files[loc.String()]
	if t, link := m.links[loc.String()]; link {
		b, ok = m.files[t]
	}
	if !ok {
		return nil, os.ErrNotExist
	}
	return memReader{bytes.NewReader(b)}, nil
}

type memWriter struct {
	bytes.Buffer
	done func([]byte)
}

func (w *memWriter) Close() error { w.done(w.Bytes()); return nil }

func (m *memRemote) OpenWrite(_ context.Context, loc pathloc.Path, _ int64, _ fsbackend.CreateOpts) (io.WriteCloser, error) {
	return &memWriter{done: func(b []byte) {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.files[loc.String()] = append([]byte(nil), b...)
	}}, nil
}

func (m *memRemote) Mkdir(_ context.Context, dir pathloc.Path, _ fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dirs[dir.String()] = true
	return nil
}

func (m *memRemote) Remove(_ context.Context, loc pathloc.Path) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, loc.String())
	delete(m.dirs, loc.String())
	return nil
}

func (m *memRemote) Rename(context.Context, pathloc.Path, pathloc.Path) error { return fs.ErrInvalid }
func (m *memRemote) ReadSymlink(context.Context, pathloc.Path) (string, error) {
	return "", fs.ErrInvalid
}
func (m *memRemote) Symlink(context.Context, pathloc.Path, string) error { return fs.ErrInvalid }

func (m *memRemote) file(path string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.files[memRemoteHost+path]
	return string(b), ok
}

// tarGzBytes packs member=content into a .tar.gz and returns its bytes.
func tarGzBytes(t *testing.T, member, content string) []byte {
	t.Helper()
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("tar not in PATH")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, member), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "pack.tar.gz")
	if err := exec.Command("tar", "-czf", out, "-C", dir, member).Run(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// scratchTmp points the temporary directory at an empty one and returns it, so a test can assert
// that the extract left nothing behind.
func scratchTmp(t *testing.T) string {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	return dir
}

func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Fatalf("temporary files left behind: %v", left)
	}
}

func TestExtractRemoteArchiveToLocalDestination(t *testing.T) {
	tc := archive.ProbeToolchain()
	mem := newMemRemote("/quarry")
	mem.files[memRemoteHost+"/quarry/orchard.tar.gz"] = tarGzBytes(t, "heron.txt", "marsh")
	installOpsBackend(t, mem)
	tmp, dest := scratchTmp(t), t.TempDir()

	plan, _, err := PlanExtract([]string{memRemoteHost + "/quarry/orchard.tar.gz"}, dest, tc)
	if err != nil {
		t.Fatal(err)
	}
	plan.AsDirectory = true
	if done, err := ExecuteExtract(context.Background(), plan, nil); err != nil || done != 1 {
		t.Fatalf("done=%d err=%v", done, err)
	}
	if b, err := os.ReadFile(filepath.Join(dest, "orchard", "heron.txt")); err != nil || string(b) != "marsh" {
		t.Fatalf("extracted file = %q, %v", b, err)
	}
	assertEmptyDir(t, tmp)
}

func TestExtractLocalArchiveToRemoteDestination(t *testing.T) {
	tc := archive.ProbeToolchain()
	mem := newMemRemote("/thicket")
	installOpsBackend(t, mem)
	tmp, srcDir := scratchTmp(t), t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "meadow.tar.gz"), tarGzBytes(t, "willow.txt", "reed"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeGzipFile(t, filepath.Join(srcDir, "lantern.gz"), "moss")

	for _, asDir := range []bool{false, true} {
		plan, _, err := PlanExtract([]string{filepath.Join(srcDir, "meadow.tar.gz"), filepath.Join(srcDir, "lantern.gz")}, memRemoteHost+"/thicket", tc)
		if err != nil {
			t.Fatal(err)
		}
		plan.AsDirectory = asDir
		if done, err := ExecuteExtract(context.Background(), plan, nil); err != nil || done != 2 {
			t.Fatalf("asDir=%v done=%d err=%v", asDir, done, err)
		}
		want := "/thicket/willow.txt"
		if asDir {
			want = "/thicket/meadow/willow.txt"
		}
		if got, ok := mem.file(want); !ok || got != "reed" {
			t.Fatalf("asDir=%v %s = %q, %v", asDir, want, got, ok)
		}
		if got, ok := mem.file("/thicket/lantern"); !ok || got != "moss" { // a stream format ignores asDir
			t.Fatalf("asDir=%v stream output = %q, %v", asDir, got, ok)
		}
		mem.files = map[string][]byte{}
		mem.dirs = map[string]bool{memRemoteHost: true, memRemoteHost + "/thicket": true}
	}
	assertEmptyDir(t, tmp)
}

func TestExtractRemoteToRemoteHonorsConflictResolver(t *testing.T) {
	tc := archive.ProbeToolchain()
	mem := newMemRemote("/quarry", "/thicket")
	mem.files[memRemoteHost+"/quarry/orchard.tar.gz"] = tarGzBytes(t, "heron.txt", "marsh")
	mem.files[memRemoteHost+"/thicket/heron.txt"] = []byte("keep-harbor")
	installOpsBackend(t, mem)
	tmp := scratchTmp(t)

	plan, _, err := PlanExtract([]string{memRemoteHost + "/quarry/orchard.tar.gz"}, memRemoteHost+"/thicket", tc)
	if err != nil {
		t.Fatal(err)
	}
	asked := 0
	plan.Conflict = func(_ context.Context, _, dst string, _ FileConflictFacts) (ConflictResolution, error) {
		asked++
		if !strings.HasSuffix(dst, "/thicket/heron.txt") {
			t.Errorf("conflict on %s", dst)
		}
		return ConflictResolution{Action: ActionSkip}, nil
	}
	if _, err := ExecuteExtract(context.Background(), plan, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := mem.file("/thicket/heron.txt"); asked != 1 || got != "keep-harbor" {
		t.Fatalf("asked=%d existing file = %q", asked, got)
	}
	assertEmptyDir(t, tmp)
}

func TestExtractRemoteArchivePasswordNamesRemotePath(t *testing.T) {
	tc := archive.ProbeToolchain()
	if _, err := exec.LookPath("zip"); err != nil || !archive.FormatZip.Available(tc) {
		t.Skip("zip/unzip not available")
	}
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "reed.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("zip", "-q", "-P", "willow", "marsh.zip", "reed.txt")
	cmd.Dir = srcDir
	if err := cmd.Run(); err != nil {
		t.Skipf("cannot create encrypted zip: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(srcDir, "marsh.zip"))
	if err != nil {
		t.Fatal(err)
	}
	mem := newMemRemote("/quarry")
	mem.files[memRemoteHost+"/quarry/marsh.zip"] = b
	installOpsBackend(t, mem)
	dest := t.TempDir()

	plan, _, err := PlanExtract([]string{memRemoteHost + "/quarry/marsh.zip"}, dest, tc)
	if err != nil {
		t.Fatal(err)
	}
	var asked string
	plan.Password = func(_ context.Context, path string, _ bool) (string, bool) { asked = path; return "willow", true }
	if done, err := ExecuteExtract(context.Background(), plan, nil); err != nil || done != 1 {
		t.Fatalf("done=%d err=%v", done, err)
	}
	if asked != memRemoteHost+"/quarry/marsh.zip" {
		t.Fatalf("password asked for %q", asked)
	}
	if _, err := os.Stat(filepath.Join(dest, "reed.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestPlanExtractForJobKeepsExistingRemoteStreamOutput(t *testing.T) {
	tc := requireGzipToolchain(t)
	mem := newMemRemote("/thicket")
	mem.files[memRemoteHost+"/thicket/lantern"] = []byte("keep-harbor")
	installOpsBackend(t, mem)
	srcDir := t.TempDir()
	writeGzipFile(t, filepath.Join(srcDir, "lantern.gz"), "moss")
	archives := []string{filepath.Join(srcDir, "lantern.gz")}

	if _, skipped, err := PlanExtract(archives, memRemoteHost+"/thicket", tc); err == nil || len(skipped) != 1 {
		t.Fatalf("PlanExtract should skip the existing output: skipped=%v err=%v", skipped, err)
	}
	plan, _, err := PlanExtractForJob(context.Background(), archives, memRemoteHost+"/thicket", tc)
	if err != nil || len(plan.Items) != 1 || plan.Destination != memRemoteHost+"/thicket" {
		t.Fatalf("PlanExtractForJob: %+v %v", plan, err)
	}
}

func TestExtractRemoteSymlinkToArchive(t *testing.T) {
	tc := archive.ProbeToolchain()
	mem := newMemRemote("/quarry", "/thicket")
	mem.files[memRemoteHost+"/quarry/orchard.tar.gz"] = tarGzBytes(t, "heron.txt", "marsh")
	mem.links[memRemoteHost+"/quarry/alias.tar.gz"] = memRemoteHost + "/quarry/orchard.tar.gz"
	mem.links[memRemoteHost+"/quarry/dangling.tar.gz"] = memRemoteHost + "/quarry/missing.tar.gz"
	mem.links[memRemoteHost+"/thicketlink"] = memRemoteHost + "/thicket"
	installOpsBackend(t, mem)
	tmp, dest := scratchTmp(t), t.TempDir()

	if e, _ := mem.Stat(context.Background(), pathloc.MustParse(memRemoteHost+"/quarry/alias.tar.gz")); e.Type != fsbackend.EntrySymlink {
		t.Fatalf("Stat must keep lstat semantics, got type %v", e.Type)
	}
	if _, skipped, err := PlanExtract([]string{memRemoteHost + "/quarry/dangling.tar.gz"}, dest, tc); err == nil || len(skipped) != 1 {
		t.Fatalf("dangling link should be skipped: skipped=%v err=%v", skipped, err)
	}
	// The destination may be a symlink to a directory as well.
	plan, _, err := PlanExtract([]string{memRemoteHost + "/quarry/alias.tar.gz"}, memRemoteHost+"/thicketlink", tc)
	if err != nil {
		t.Fatal(err)
	}
	if done, err := ExecuteExtract(context.Background(), plan, nil); err != nil || done != 1 {
		t.Fatalf("remote destination: done=%d err=%v", done, err)
	}
	if got, ok := mem.file("/thicketlink/heron.txt"); !ok || got != "marsh" { // written through the link
		t.Fatalf("uploaded file = %q, %v", got, ok)
	}
	plan, _, err = PlanExtract([]string{memRemoteHost + "/quarry/alias.tar.gz"}, dest, tc)
	if err != nil {
		t.Fatal(err)
	}
	if done, err := ExecuteExtract(context.Background(), plan, nil); err != nil || done != 1 {
		t.Fatalf("done=%d err=%v", done, err)
	}
	if b, err := os.ReadFile(filepath.Join(dest, "heron.txt")); err != nil || string(b) != "marsh" {
		t.Fatalf("extracted file = %q, %v", b, err)
	}
	assertEmptyDir(t, tmp)
}
