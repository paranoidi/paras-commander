//go:build linux

package findcrit

import (
	"io/fs"
	"syscall"
)

// MetaFromInfo builds an entry's Meta from its lstat result.
func MetaFromInfo(info fs.FileInfo) *Meta {
	m := &Meta{ModTime: info.ModTime().UnixNano(), Mode: info.Mode()}
	m.ChangeTime, m.AccessTime = m.ModTime, m.ModTime
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		m.ChangeTime = st.Ctim.Sec*1e9 + st.Ctim.Nsec
		m.AccessTime = st.Atim.Sec*1e9 + st.Atim.Nsec
		m.UID = st.Uid
	}
	return m
}
