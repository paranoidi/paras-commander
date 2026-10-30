// Package foldercolor reads the folder colours that Linux Mint's Nemo (folder-color-switcher)
// stores in the GVfs metadata database and writes them back through gio.
package foldercolor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"maps"
	"strings"
)

// iconRef is the colour-bearing metadata of one path: metadata::custom-icon (an icon file URI)
// and metadata::custom-icon-name (an icon name such as "folder-red").
type iconRef struct{ icon, name string }

const (
	keyIcon = "custom-icon"
	keyName = "custom-icon-name"

	journalHeaderLen = 20

	jSet    = 0
	jUnset  = 2
	jCopy   = 3
	jRemove = 4

	maxTreeDepth = 64
)

var (
	treeMagic    = []byte("\xda\x1ameta")
	journalMagic = []byte("\xda\x1ajour")
	errBadTree   = errors.New("foldercolor: malformed metadata tree")
)

// treeReader reads big-endian values and NUL-terminated strings with bounds checks, so a
// truncated or concurrently rewritten file yields errors instead of panics.
type treeReader struct{ d []byte }

func (r treeReader) u32(o uint32) (uint32, bool) {
	if uint64(o)+4 > uint64(len(r.d)) {
		return 0, false
	}
	return binary.BigEndian.Uint32(r.d[o:]), true
}

func (r treeReader) str(o uint32) (string, bool) {
	if uint64(o) >= uint64(len(r.d)) {
		return "", false
	}
	n := bytes.IndexByte(r.d[o:], 0)
	if n < 0 {
		return "", false
	}
	return string(r.d[o : int(o)+n]), true
}

// readTree decodes the tree file into home-relative path -> iconRef (only paths carrying one of
// the two keys) and returns the tree's random_tag, which names the matching journal.
func readTree(data []byte) (map[string]iconRef, uint32, error) {
	r := treeReader{data}
	if len(data) < 0x20 || !bytes.HasPrefix(data, treeMagic) {
		return nil, 0, errBadTree
	}
	tag, _ := r.u32(0xC)
	root, _ := r.u32(0x10)
	attrs, _ := r.u32(0x14)
	nAttr, ok := r.u32(attrs)
	if !ok {
		return nil, 0, errBadTree
	}
	keys := make([]string, 0, nAttr)
	for i := uint32(0); i < nAttr; i++ {
		so, ok := r.u32(attrs + 4 + 4*i)
		if !ok {
			return nil, 0, errBadTree
		}
		s, ok := r.str(so)
		if !ok {
			return nil, 0, errBadTree
		}
		keys = append(keys, s)
	}
	out := map[string]iconRef{}
	var walk func(ent uint32, path string, depth int)
	walk = func(ent uint32, path string, depth int) {
		if depth > maxTreeDepth {
			return
		}
		if md, _ := r.u32(ent + 8); md != 0 {
			n, _ := r.u32(md)
			var ref iconRef
			for i := uint32(0); i < n; i++ {
				k, ok1 := r.u32(md + 4 + 8*i)
				v, ok2 := r.u32(md + 8 + 8*i)
				if !ok1 || !ok2 || k&0x80000000 != 0 || int(k) >= len(keys) {
					continue
				}
				switch keys[k] {
				case keyIcon:
					ref.icon, _ = r.str(v)
				case keyName:
					ref.name, _ = r.str(v)
				}
			}
			if ref != (iconRef{}) {
				out[path] = ref
			}
		}
		if ch, _ := r.u32(ent + 4); ch != 0 {
			n, _ := r.u32(ch)
			for i := uint32(0); i < n; i++ {
				child := ch + 4 + 16*i
				no, ok := r.u32(child)
				if !ok {
					return
				}
				name, ok := r.str(no)
				if !ok {
					return
				}
				p := name
				if path != "" {
					p = path + "/" + name
				}
				walk(child, p, depth+1)
			}
		}
	}
	walk(root, "", 0)
	return out, tag, nil
}

// journalEntryCount returns num_entries from a journal header, or false when it is not a journal.
func journalEntryCount(data []byte) (uint32, bool) {
	if len(data) < journalHeaderLen || !bytes.HasPrefix(data, journalMagic) {
		return 0, false
	}
	return binary.BigEndian.Uint32(data[0x10:]), true
}

// replayJournal applies the journal's SET / UNSET / COPY / REMOVE entries for the two colour keys
// on top of m. Entry layout (verified against gvfsd-metadata output): entry_size u32, crc32 u32,
// mtime u64, type u8, NUL-terminated strings, padding, entry_size u32 again; entry_size spans the
// whole entry. Paths carry a leading "/". A journal whose random_tag differs from the tree's
// belongs to another tree generation and is ignored.
func replayJournal(data []byte, tag uint32, m map[string]iconRef) {
	n, ok := journalEntryCount(data)
	if !ok || binary.BigEndian.Uint32(data[8:]) != tag {
		return
	}
	pos := uint32(journalHeaderLen)
	for i := uint32(0); i < n; i++ {
		if uint64(pos)+4 > uint64(len(data)) {
			return
		}
		size := binary.BigEndian.Uint32(data[pos:])
		if size < 21 || uint64(pos)+uint64(size) > uint64(len(data)) {
			return
		}
		body := data[pos+16 : pos+size-4]
		pos += size
		typ := body[0]
		f := strings.Split(string(body[1:]), "\x00")
		rel := func(s string) string { return strings.Trim(s, "/") }
		switch {
		case (typ == jSet && len(f) >= 3) || (typ == jUnset && len(f) >= 2):
			val := "" // UNSET clears the key
			if typ == jSet {
				val = f[2]
			}
			p := rel(f[0])
			ref := m[p]
			switch f[1] {
			case keyIcon:
				ref.icon = val
			case keyName:
				ref.name = val
			default:
				continue
			}
			if ref == (iconRef{}) {
				delete(m, p)
			} else {
				m[p] = ref
			}
		case typ == jRemove && len(f) >= 1:
			dropSubtree(m, rel(f[0]))
		case typ == jCopy && len(f) >= 2:
			dst, src := rel(f[0]), rel(f[1])
			dropSubtree(m, dst)
			moved := map[string]iconRef{}
			for p, ref := range m {
				if p == src || strings.HasPrefix(p, src+"/") {
					moved[dst+strings.TrimPrefix(p, src)] = ref
				}
			}
			maps.Copy(m, moved)
		}
	}
}

// dropSubtree deletes p and everything below it.
func dropSubtree(m map[string]iconRef, p string) {
	for k := range m {
		if k == p || strings.HasPrefix(k, p+"/") {
			delete(m, k)
		}
	}
}
