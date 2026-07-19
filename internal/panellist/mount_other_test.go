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

func TestEntryOnOtherMountUsesCachedDevice(t *testing.T) {
	dir := localfs.Entry{Name: "media", Path: "/srv/media", Type: localfs.EntryDirectory, Dev: 7, DevValid: true}
	if !EntryOnOtherMount(dir, false, 1, true) {
		t.Fatal("directory with different cached device should be other-mount")
	}
	same := localfs.Entry{Name: "docs", Path: "/srv/docs", Type: localfs.EntryDirectory, Dev: 1, DevValid: true}
	if EntryOnOtherMount(same, false, 1, true) {
		t.Fatal("directory on the listing device should not be other-mount")
	}
}

func TestEntryOnOtherMountWhenEntryDeviceUnknown(t *testing.T) {
	dir := localfs.Entry{Name: "mnt", Path: "/mnt/nas", Type: localfs.EntryDirectory}
	if EntryOnOtherMount(dir, false, 1, true) {
		t.Fatal("entry without cached device must not be other-mount (and must not stat)")
	}
}
