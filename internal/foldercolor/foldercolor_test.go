package foldercolor

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// buf is a tiny big-endian writer that lays out GVfs metadata structures at known offsets.
type buf struct{ b bytes.Buffer }

func (w *buf) u32(v uint32) { _ = binary.Write(&w.b, binary.BigEndian, v) }
func (w *buf) off() uint32  { return uint32(w.b.Len()) }
func (w *buf) str(s string) uint32 {
	o := w.off()
	w.b.WriteString(s)
	w.b.WriteByte(0)
	return o
}

type fixtureDir struct {
	name     string
	icon     string // custom-icon value ("" = none)
	children []fixtureDir
}

// encodeTree builds a tree file: header, attribute stringv, then dir entries bottom-up.
func encodeTree(tag uint32, root []fixtureDir) []byte {
	w := &buf{}
	w.b.Write(treeMagic)
	w.b.Write([]byte{1, 0})
	w.u32(0) // rotated
	w.u32(tag)
	w.u32(0) // root, patched below
	w.u32(0) // attributes, patched below
	w.u32(0)
	w.u32(0) // time_t_base

	kIcon := w.str(keyIcon)
	kOther := w.str("unrelated")
	attrs := w.off()
	w.u32(2)
	w.u32(kIcon)
	w.u32(kOther)

	// ent writes the metadata and children of d and returns the offset of its 16-byte DirEnt.
	var ent func(name string, d []fixtureDir, icon string) uint32
	ent = func(name string, kids []fixtureDir, icon string) uint32 {
		var md, ch uint32
		nameOff := w.str(name)
		if icon != "" {
			v := w.str(icon)
			md = w.off()
			w.u32(2)
			w.u32(0)
			w.u32(v)
			w.u32(1) // an unrelated key that must be ignored
			w.u32(v)
		}
		var offs []uint32
		for _, k := range kids {
			offs = append(offs, ent(k.name, k.children, k.icon))
		}
		if len(kids) > 0 {
			// children are stored as consecutive DirEnts, so copy the already written ones in.
			ch = w.off()
			w.u32(uint32(len(kids)))
			for _, o := range offs {
				w.b.Write(append([]byte(nil), w.b.Bytes()[o:o+16]...))
			}
		}
		e := w.off()
		w.u32(nameOff)
		w.u32(ch)
		w.u32(md)
		w.u32(0)
		return e
	}
	rootEnt := ent("/", root, "")
	out := w.b.Bytes()
	binary.BigEndian.PutUint32(out[0x10:], rootEnt)
	binary.BigEndian.PutUint32(out[0x14:], attrs)
	return out
}

type jEntry struct {
	typ    byte
	fields []string
}

func encodeJournal(tag uint32, entries ...jEntry) []byte {
	w := &buf{}
	w.b.Write(journalMagic)
	w.b.Write([]byte{1, 0})
	w.u32(tag)
	w.u32(32768)
	w.u32(uint32(len(entries)))
	for _, e := range entries {
		var body bytes.Buffer
		body.WriteByte(e.typ)
		for _, f := range e.fields {
			body.WriteString(f)
			body.WriteByte(0)
		}
		for body.Len()%4 != 3 { // 16 header + body + 4 trailer must be 4-aligned
			body.WriteByte(0)
		}
		size := uint32(16 + body.Len() + 4)
		w.u32(size)
		w.u32(0) // crc32 (not verified)
		w.u32(0)
		w.u32(uint32(time.Now().Unix()))
		w.b.Write(body.Bytes())
		w.u32(size)
	}
	out := w.b.Bytes()
	return append(out, make([]byte, 32768-len(out))...)
}

func writeFixturePalette(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	js := `{"styles":[{"name":"Mint-Y","icon-themes":[
		{"name":"Blue","color":"#5294e2","theme":"Mint-Y-Blue"},
		{"name":"Green","color":"#50c16f","theme":"Mint-Y"},
		{"name":"Red","color":"#f54f54","theme":"Mint-Y-Red"}]}]}`
	if err := os.WriteFile(filepath.Join(dir, "Mint-Y.json"), []byte(js), 0o600); err != nil {
		t.Fatal(err)
	}
	old := colorsDir
	colorsDir = dir
	t.Cleanup(func() { colorsDir = old })
}

func uri(theme, file string) string {
	return "file:///usr/share/icons/" + theme + "/places/48/" + file + ".png"
}

func TestReadTree(t *testing.T) {
	data := encodeTree(0xabcd, []fixtureDir{
		{name: "harbor", icon: uri("Mint-Y-Red", "folder"), children: []fixtureDir{
			{name: "lantern", icon: uri("Mint-Y-Blue", "folder-documents")},
			{name: "plain"},
		}},
		{name: "meadow"},
	})
	got, tag, err := readTree(data)
	if err != nil {
		t.Fatal(err)
	}
	if tag != 0xabcd {
		t.Fatalf("tag %x", tag)
	}
	if len(got) != 2 || got["harbor"].icon != uri("Mint-Y-Red", "folder") || got["harbor/lantern"].icon != uri("Mint-Y-Blue", "folder-documents") {
		t.Fatalf("unexpected refs: %#v", got)
	}
	if _, _, err := readTree(data[:40]); err == nil {
		t.Fatal("truncated tree should fail")
	}
}

func TestReplayJournal(t *testing.T) {
	m := map[string]iconRef{
		"orchard":        {icon: "a"},
		"orchard/cobalt": {icon: "b"},
		"quarry":         {icon: "c"},
	}
	j := encodeJournal(7,
		jEntry{jSet, []string{"/velvet", keyIcon, "d"}},
		jEntry{jSet, []string{"/velvet", "other", "zzz"}},
		jEntry{jUnset, []string{"/quarry", keyIcon}},
		jEntry{jCopy, []string{"/pasture", "/orchard"}},
		jEntry{jRemove, []string{"/orchard"}},
	)
	replayJournal(j, 7, m)
	want := map[string]iconRef{"velvet": {icon: "d"}, "pasture": {icon: "a"}, "pasture/cobalt": {icon: "b"}}
	if len(m) != len(want) {
		t.Fatalf("got %#v", m)
	}
	for k, v := range want {
		if m[k] != v {
			t.Fatalf("%s: got %#v want %#v (all %#v)", k, m[k], v, m)
		}
	}
	// A journal of another tree generation is ignored.
	other := map[string]iconRef{}
	replayJournal(j, 8, other)
	if len(other) != 0 {
		t.Fatalf("stale journal applied: %#v", other)
	}
}

func TestPaletteResolve(t *testing.T) {
	writeFixturePalette(t)
	p := loadPalette()
	red := tcell.GetColor("#f54f54")
	if c, ok := p.color(iconRef{icon: uri("Mint-Y-Red", "folder")}); !ok || c != red {
		t.Fatalf("uri: %v %v", c, ok)
	}
	if c, ok := p.color(iconRef{name: "folder-red"}); !ok || c != red {
		t.Fatalf("name: %v %v", c, ok)
	}
	if c, ok := p.color(iconRef{icon: uri("Mint-Y", "folder")}); !ok || c != tcell.GetColor("#50c16f") {
		t.Fatalf("base theme should be green: %v %v", c, ok)
	}
	if _, ok := p.color(iconRef{icon: uri("Papirus", "folder")}); ok {
		t.Fatal("unknown theme coloured")
	}
	if got := p.styleFor("Mint-Y-Sand"); got != "Mint-Y" {
		t.Fatalf("style %q", got)
	}
}

func TestStoreReloadAndRefresh(t *testing.T) {
	writeFixturePalette(t)
	home, meta := t.TempDir(), t.TempDir()
	s := &Store{home: home, dir: meta}
	tree := encodeTree(5, []fixtureDir{{name: "tundra", icon: uri("Mint-Y-Blue", "folder")}})
	if err := os.WriteFile(s.treePath(homeTree), tree, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.journalPath(homeTree, 5), encodeJournal(5), 0o600); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	if c, ok := s.Lookup(filepath.Join(home, "tundra")); !ok || c != tcell.GetColor("#5294e2") {
		t.Fatalf("lookup: %v %v", c, ok)
	}
	if s.Refresh() {
		t.Fatal("unchanged files reported as changed")
	}
	j := encodeJournal(5, jEntry{jSet, []string{"/glacier", keyName, "folder-red"}})
	if err := os.WriteFile(s.journalPath(homeTree, 5), j, 0o600); err != nil {
		t.Fatal(err)
	}
	if !s.Refresh() {
		t.Fatal("journal rewrite not detected")
	}
	if c, ok := s.Lookup(filepath.Join(home, "glacier")); !ok || c != tcell.GetColor("#f54f54") {
		t.Fatalf("journal lookup: %v %v", c, ok)
	}
	if s.Refresh() {
		t.Fatal("still changed after reload")
	}
}

// fakeDisks points the mount table and udev links at fixtures. links maps "uuid-<n>" / "label-<n>"
// to a "major:minor"; lines are mountinfo lines.
func fakeDisks(t *testing.T, links map[string]string, lines ...string) (setMounts func(...string)) {
	t.Helper()
	dir := t.TempDir()
	byName := map[string]string{}
	for tree, dev := range links {
		kind, name, _ := strings.Cut(tree, "-")
		p := filepath.Join(dir, "disk", "by-"+kind, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		byName[p] = dev
	}
	oldM, oldD, oldN := mountInfoPath, diskDir, devNumOf
	mountInfoPath, diskDir = filepath.Join(dir, "mountinfo"), filepath.Join(dir, "disk")
	devNumOf = func(p string) (string, bool) { d, ok := byName[p]; return d, ok }
	t.Cleanup(func() { mountInfoPath, diskDir, devNumOf = oldM, oldD, oldN })
	setMounts = func(l ...string) {
		if err := os.WriteFile(mountInfoPath, []byte(strings.Join(l, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	setMounts(lines...)
	return setMounts
}

func mountLine(dev, root, point string) string {
	return "36 25 " + dev + " " + root + " " + point + " rw,relatime shared:1 - ext4 /dev/x rw"
}

func writeTree(t *testing.T, s *Store, tree string, root ...fixtureDir) {
	t.Helper()
	if err := os.WriteFile(s.treePath(tree), encodeTree(9, root), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestStoreRootAndDeviceTrees(t *testing.T) {
	writeFixturePalette(t)
	blue, red := uri("Mint-Y-Blue", "folder"), uri("Mint-Y-Red", "folder")
	green := uri("Mint-Y", "folder")
	s := &Store{home: "/nowhere/home", dir: t.TempDir()}
	// root: absolute paths; device trees: paths inside the filesystem.
	writeTree(t, s, "root",
		fixtureDir{name: "silver", children: []fixtureDir{{name: "pond", icon: blue}}},
		fixtureDir{name: "nowhere", children: []fixtureDir{{name: "home", children: []fixtureDir{{name: "both", icon: blue}}}}})
	writeTree(t, s, "uuid-AAAA-1111", fixtureDir{name: "maple", icon: red}, fixtureDir{name: "sub", children: []fixtureDir{{name: "cedar", icon: red}}}, fixtureDir{name: "other", icon: red})
	writeTree(t, s, "label-quiet_disk", fixtureDir{name: "birch", icon: green})
	writeTree(t, s, "uuid-GONE", fixtureDir{name: "willow", icon: green})
	writeTree(t, s, "home", fixtureDir{name: "both", icon: green})
	fakeDisks(t, map[string]string{"uuid-AAAA-1111": "8:1", "label-AAAA-1111": "8:1", "label-quiet_disk": "8:2", "uuid-GONE": "8:9"},
		mountLine("8:1", "/", "/mnt/key"),
		mountLine("8:1", "/sub", `/mnt/two\040words`),
		mountLine("8:2", "/", "/mnt/quiet"),
		mountLine("0:44", "/", "/mnt/tmp"),
		mountLine("8:3", "/", "/"),
	)
	s.Reload()
	want := map[string]string{
		"/silver/pond":         "#5294e2", // root
		"/mnt/key/maple":       "#f54f54", // uuid tree
		"/mnt/key/sub/cedar":   "#f54f54",
		"/mnt/two words/cedar": "#f54f54", // second mount of the same filesystem, root "/sub", escaped mount point
		"/mnt/quiet/birch":     "#50c16f", // label tree, no by-uuid link for the device
	}
	for p, hex := range want {
		if c, ok := s.Lookup(p); !ok || c != tcell.GetColor(hex) {
			t.Errorf("%s: %v %v", p, c, ok)
		}
	}
	for _, p := range []string{"/mnt/two words/other", "/mnt/two words/maple", "/mnt/key/willow", "/mnt/tmp/willow"} {
		if _, ok := s.Lookup(p); ok {
			t.Errorf("%s should not be coloured", p)
		}
	}
	// home beats root for the same absolute path.
	s.Reload()
	if c, _ := s.Lookup("/nowhere/home/both"); c != tcell.GetColor("#50c16f") {
		t.Errorf("precedence: %v", c)
	}
}

func TestDevicePrecedenceAndRefresh(t *testing.T) {
	writeFixturePalette(t)
	s := &Store{home: "/nowhere/home", dir: t.TempDir()}
	writeTree(t, s, "root", fixtureDir{name: "mnt", children: []fixtureDir{{name: "key", children: []fixtureDir{{name: "maple", icon: uri("Mint-Y-Blue", "folder")}}}}})
	writeTree(t, s, "uuid-AAAA", fixtureDir{name: "maple", icon: uri("Mint-Y-Red", "folder")})
	setMounts := fakeDisks(t, map[string]string{"uuid-AAAA": "8:1"})
	s.Reload()
	if c, _ := s.Lookup("/mnt/key/maple"); c != tcell.GetColor("#5294e2") {
		t.Fatalf("unmounted device read: %v", c)
	}
	if s.Refresh() {
		t.Fatal("changed without a change")
	}
	setMounts(mountLine("8:1", "/", "/mnt/key"))
	if !s.Refresh() {
		t.Fatal("mount not detected")
	}
	if c, _ := s.Lookup("/mnt/key/maple"); c != tcell.GetColor("#f54f54") {
		t.Fatalf("device should beat root: %v", c)
	}
	if s.Refresh() {
		t.Fatal("changed after reload")
	}
}

func TestUnescapeMount(t *testing.T) {
	if got := unescapeMount(`/mnt/a\040b\134c`); got != `/mnt/a b\c` {
		t.Fatalf("%q", got)
	}
	if got := unescapeMount(`/mnt/plain`); got != `/mnt/plain` {
		t.Fatal(got)
	}
}

// TestApplyGio drives the real gio and gvfsd-metadata: set a colour on a scratch dir under $HOME,
// see it through Reload, then clear it. Skipped without gio, a palette, or a metadata daemon.
func TestApplyGio(t *testing.T) {
	if _, err := exec.LookPath("gio"); err != nil {
		t.Skip("gio not installed")
	}
	if _, err := os.Stat(colorsDir); err != nil {
		t.Skip("no folder-color-switcher palette")
	}
	s := New()
	if _, err := os.Stat(s.dir); err != nil {
		t.Skip("no gvfs metadata")
	}
	cache, err := os.UserCacheDir()
	if rel, rerr := filepath.Rel(s.home, cache); err != nil || rerr != nil || !filepath.IsLocal(rel) {
		t.Skip("cache dir not under $HOME")
	}
	dir, err := os.MkdirTemp(cache, "pc-foldercolor-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = exec.Command("gio", "remove", dir).Run(); _ = os.RemoveAll(dir) })

	s.Reload()
	var red *Choice
	for _, c := range s.Choices() {
		if c.Name == "Red" {
			red = &c
		}
	}
	if red == nil {
		t.Skip("no Red choice for the current icon theme")
	}
	if err := s.Apply([]string{dir}, red); err != nil {
		t.Skipf("gio set failed (no gvfs daemon?): %v", err)
	}
	waitFor(t, s, func() bool { _, ok := s.Lookup(dir); return ok })
	if err := s.Apply([]string{dir}, nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, s, func() bool { _, ok := s.Lookup(dir); return !ok })
}

func waitFor(t *testing.T, s *Store, cond func() bool) {
	t.Helper()
	for range 40 {
		s.Reload()
		if cond() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("lookup state did not change in time")
}

func TestWatchIconTheme(t *testing.T) {
	dir := t.TempDir()
	js := `{"styles":[
		{"name":"Mint-Y","icon-themes":[{"name":"Blue","color":"#5294e2","theme":"Mint-Y-Blue"}]},
		{"name":"Mint-L","icon-themes":[{"name":"Sand","color":"#c8ac69","theme":"Mint-L-Sand"}]}]}`
	if err := os.WriteFile(filepath.Join(dir, "Mint.json"), []byte(js), 0o600); err != nil {
		t.Fatal(err)
	}
	// Fake gsettings: "get" answers Mint-Y, "monitor" reports a switch to Mint-L and stays alive.
	script := filepath.Join(dir, "gsettings")
	body := "#!/bin/sh\nif [ \"$1\" = get ]; then echo \"'Mint-Y'\"; exit; fi\necho \"icon-theme: 'Mint-L-Sand'\"\nexec sleep 30\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	oldC, oldG := colorsDir, gsettingsBin
	colorsDir, gsettingsBin = dir, script
	t.Cleanup(func() { colorsDir, gsettingsBin = oldC, oldG })

	s := &Store{home: t.TempDir(), dir: t.TempDir()}
	s.Reload()
	if got := s.Choices(); len(got) != 1 || got[0].Name != "Blue" {
		t.Fatalf("initial choices %v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	changes := make(chan struct{}, 4)
	done := make(chan struct{})
	go func() { s.WatchIconTheme(ctx, func() { changes <- struct{}{} }); close(done) }()
	// The startup read matches the first load (Mint-Y) and must not report; the monitor line must.
	select {
	case <-changes:
	case <-time.After(5 * time.Second):
		t.Fatal("no theme change reported")
	}
	if got := s.Choices(); len(got) != 1 || got[0].Name != "Sand" {
		t.Fatalf("watched choices %v", got)
	}
	s.Reload() // must keep the watched theme, not re-read "get"
	if got := s.Choices(); got[0].Name != "Sand" {
		t.Fatalf("reload lost the watched theme: %v", got)
	}
	cancel()
	<-done
	if n := len(changes); n != 0 {
		t.Fatalf("%d extra change reports, want only the monitor's", n)
	}
	if s.watched {
		t.Fatal("still marked watched after the monitor stopped")
	}
}
