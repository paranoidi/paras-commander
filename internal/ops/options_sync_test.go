package ops

import "testing"

func TestSyncFilePolicy(t *testing.T) {
	t.Parallel()
	base := Options{
		SyncAfterEachFile: true,
		SyncAtJobEnd:      false,
		SyncMinFileKiB:    0,
	}
	if !base.SyncFileNow(1024) {
		t.Fatal("expected per-file sync when enabled")
	}
	if base.SyncFileDeferred(1024) {
		t.Fatal("deferred sync should be off when per-file sync is on")
	}

	deferred := Options{
		SyncAfterEachFile: false,
		SyncAtJobEnd:      true,
		SyncMinFileKiB:    0,
	}
	if deferred.SyncFileNow(1024) {
		t.Fatal("per-file sync should be off")
	}
	if !deferred.SyncFileDeferred(1024) {
		t.Fatal("expected deferred sync at job end")
	}

	threshold := Options{
		SyncAfterEachFile: true,
		SyncMinFileKiB:    64,
	}
	if threshold.SyncFileNow(32 * 1024) {
		t.Fatal("file below threshold should skip sync")
	}
	if !threshold.SyncFileNow(128 * 1024) {
		t.Fatal("file at/above threshold should sync")
	}
}
