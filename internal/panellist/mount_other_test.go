package panellist

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestEntryOnOtherMountSkipsFiles(t *testing.T) {
	file := localfs.Entry{Name: "readme.txt", Path: "/tmp/readme.txt", Type: localfs.EntryFile}
	if EntryOnOtherMount(file, false, 1, true) {
		t.Fatal("file entry should not be other-mount")
	}
}

func TestEntryOnOtherMountWhenDescendEnabled(t *testing.T) {
	dir := localfs.Entry{Name: "mnt", Path: "/mnt/nas", Type: localfs.EntryDirectory}
	if EntryOnOtherMount(dir, true, 1, true) {
		t.Fatal("descendIntoMountPoints disables other-mount marking")
	}
}

func TestEntryOnOtherMountWhenListingDeviceUnknown(t *testing.T) {
	dir := localfs.Entry{Name: "mnt", Path: "/mnt/nas", Type: localfs.EntryDirectory}
	if EntryOnOtherMount(dir, false, 0, false) {
		t.Fatal("unknown listing device disables other-mount marking")
	}
}
