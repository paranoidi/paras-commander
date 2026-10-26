// Package oplog keeps one job's per-file operation records (done, skipped, failed) for the
// jobs view Operations pane. Records live in memory up to a configurable row count; past it
// the log spills to a file in the session cache directory and keeps only the newest rows in
// memory, so a job of millions of files stays bounded. Indexes (counts, per-result record
// lists, file checkpoints) always stay in memory. See ADR 0021.
package oplog

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sync"
)

// Result is the outcome of one operation record.
type Result uint8

const (
	Done Result = iota
	Skipped
	Failed

	numResults
)

// Mask is a set of results (bit 1<<Result).
type Mask uint8

// MaskAll shows every result.
const MaskAll Mask = 1<<Done | 1<<Skipped | 1<<Failed

// Has reports whether m contains r.
func (m Mask) Has(r Result) bool { return m&(1<<r) != 0 }

// Record is one processed item: its display label (the destination path relative to the job
// destination, or the source for jobs without one) and, for skipped or failed items, why.
type Record struct {
	Result Result
	Label  string
	Reason string
}

// checkpointEvery is the record spacing of the file offset index used for seeking.
const checkpointEvery = 1024

// writeBufferBytes is the buffered writer size for the spill file.
const writeBufferBytes = 64 << 10

// Config sets how a Log keeps its records. MemoryRows is the row count kept in memory before
// spilling (0 keeps everything in memory). Session supplies the spill directory; nil means no
// spill is possible and the log drops its oldest rows past MemoryRows (Incomplete).
type Config struct {
	MemoryRows int
	Session    *Session
}

// Log is one job's operation records. All methods are safe for concurrent use; appends come
// from the job goroutine, reads from the UI and its fetch goroutines.
type Log struct {
	mu    sync.Mutex
	cfg   Config
	jobID string

	n       uint32   // records appended
	mem     []Record // records memBase..n-1
	memBase uint32

	counts        [numResults]int
	lists         [numResults][]uint32 // record numbers per result, ascending (superseded removed)
	failedByLabel map[string]uint32
	superseded    map[uint32]struct{}
	supVersion    int
	version       uint64

	merged map[Mask]*mergedView // multi-result views, extended incrementally

	file        *os.File
	path        string
	w           *bufio.Writer
	written     int64   // bytes handed to w
	checkpoints []int64 // file offset of record k*checkpointEvery
	spilled     bool    // records 0..memBase-1 are readable from the file
	broken      bool    // spill failed: records below memBase are gone
	closed      bool
}

// New returns an empty log for jobID.
func New(jobID string, cfg Config) *Log {
	return &Log{cfg: cfg, jobID: jobID, failedByLabel: map[string]uint32{}, superseded: map[uint32]struct{}{}}
}

// Append records one item. A done or skipped record supersedes (hides) an earlier failure of
// the same label, as does a newer failure, so a retried item shows only its latest outcome.
func (l *Log) Append(rec Record) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	num := l.n
	l.n++
	if old, ok := l.failedByLabel[rec.Label]; ok {
		l.supersedeUnlocked(rec.Label, old)
	}
	if rec.Result == Failed {
		l.failedByLabel[rec.Label] = num
	}
	l.lists[rec.Result] = append(l.lists[rec.Result], num)
	l.counts[rec.Result]++
	l.mem = append(l.mem, rec)
	l.version++

	switch {
	case l.spilled:
		l.writeUnlocked(num, rec)
	case !l.broken && l.cfg.MemoryRows > 0 && len(l.mem) >= l.cfg.MemoryRows:
		l.spillUnlocked()
	}
	if (l.spilled || l.broken) && len(l.mem) >= 2*l.cfg.MemoryRows {
		drop := len(l.mem) - l.cfg.MemoryRows
		l.mem = slices.Clone(l.mem[drop:])
		l.memBase += uint32(drop)
	}
}

func (l *Log) supersedeUnlocked(label string, num uint32) {
	delete(l.failedByLabel, label)
	l.superseded[num] = struct{}{}
	if i, ok := slices.BinarySearch(l.lists[Failed], num); ok {
		l.lists[Failed] = slices.Delete(l.lists[Failed], i, i+1)
		l.counts[Failed]--
	}
	l.supVersion++
}

func (l *Log) spillUnlocked() {
	path, err := l.cfg.Session.filePath(l.jobID)
	if err == nil {
		l.file, err = os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0o600)
	}
	if err != nil {
		l.broken = true
		return
	}
	l.path = path
	l.w = bufio.NewWriterSize(l.file, writeBufferBytes)
	l.spilled = true
	for i, rec := range l.mem {
		l.writeUnlocked(l.memBase+uint32(i), rec)
	}
}

func (l *Log) writeUnlocked(num uint32, rec Record) {
	if num%checkpointEvery == 0 {
		l.checkpoints = append(l.checkpoints, l.written)
	}
	var hdr [1 + 2*binary.MaxVarintLen64]byte
	hdr[0] = byte(rec.Result)
	k := 1 + binary.PutUvarint(hdr[1:], uint64(len(rec.Label)))
	buf := append(hdr[:k:k], rec.Label...)
	buf = binary.AppendUvarint(buf, uint64(len(rec.Reason)))
	buf = append(buf, rec.Reason...)
	if _, err := l.w.Write(buf); err != nil {
		l.breakDiskUnlocked()
		return
	}
	l.written += int64(len(buf))
}

// breakDiskUnlocked abandons the spill file after a write error: the job keeps running and
// the log keeps its in-memory rows, but rows older than those are lost (Incomplete).
func (l *Log) breakDiskUnlocked() {
	l.removeFileUnlocked()
	l.spilled, l.broken = false, true
}

func (l *Log) removeFileUnlocked() {
	if l.file != nil {
		_ = l.file.Close()
		_ = os.Remove(l.path)
	}
	l.file, l.w, l.path, l.checkpoints = nil, nil, "", nil
}

// Close drops the records and removes the spill file. Later appends are ignored.
func (l *Log) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	l.removeFileUnlocked()
	l.n, l.memBase, l.counts, l.version = 0, 0, [numResults]int{}, l.version+1
	l.mem, l.lists, l.merged = nil, [numResults][]uint32{}, nil
	l.failedByLabel, l.superseded = map[string]uint32{}, map[uint32]struct{}{}
	l.spilled, l.broken = false, false
}

// Len is the number of records appended (including superseded and dropped ones).
func (l *Log) Len() uint32 {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.n
}

// Stats is a snapshot of a log's counters.
type Stats struct {
	Done, Skipped, Failed int
	// Dropped is the number of oldest rows lost because the spill file could not be written.
	Dropped int
	// Version changes on every append.
	Version uint64
}

// Stats returns the log's counters.
func (l *Log) Stats() Stats {
	if l == nil {
		return Stats{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return Stats{
		Done: l.counts[Done], Skipped: l.counts[Skipped], Failed: l.counts[Failed],
		Dropped: int(l.lowestUnlocked()), Version: l.version,
	}
}

// lowestUnlocked is the oldest readable record number.
func (l *Log) lowestUnlocked() uint32 {
	if l.broken {
		return l.memBase
	}
	return 0
}

// mergedView is the ascending record numbers of a multi-result mask up to record upto. Appends
// only add larger numbers, so it is extended from the lists' tails; a supersede rebuilds it.
type mergedView struct {
	nums       []uint32
	upto       uint32
	supVersion int
}

// viewUnlocked returns the ascending record numbers of mask, or nil with all=true when the
// view is every record from lowest to n (no superseded rows hidden).
func (l *Log) viewUnlocked(mask Mask) (nums []uint32, all bool) {
	mask &= MaskAll
	if mask == MaskAll && len(l.superseded) == 0 {
		return nil, true
	}
	var parts [][]uint32
	for r := range numResults {
		if mask.Has(r) {
			parts = append(parts, l.lists[r])
		}
	}
	switch len(parts) {
	case 0:
		return []uint32{}, false
	case 1:
		return parts[0], false
	}
	if l.merged == nil {
		l.merged = map[Mask]*mergedView{}
	}
	mv := l.merged[mask]
	if mv == nil || mv.supVersion != l.supVersion {
		mv = &mergedView{supVersion: l.supVersion}
		l.merged[mask] = mv
	}
	if mv.upto < l.n {
		tails := make([][]uint32, len(parts))
		for i, p := range parts {
			j, _ := slices.BinarySearch(p, mv.upto)
			tails[i] = p[j:]
		}
		mv.nums = append(mv.nums, mergeSorted(tails)...)
		mv.upto = l.n
	}
	return mv.nums, false
}

func mergeSorted(parts [][]uint32) []uint32 {
	total := 0
	for _, p := range parts {
		total += len(p)
	}
	out := make([]uint32, 0, total)
	for _, p := range parts {
		out = append(out, p...)
	}
	slices.Sort(out)
	return out
}

// clip drops record numbers below lowest from an ascending list.
func clip(nums []uint32, lowest uint32) []uint32 {
	i, _ := slices.BinarySearch(nums, lowest)
	return nums[i:]
}

// ViewLen is the number of readable records whose result is in mask.
func (l *Log) ViewLen(mask Mask) int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	nums, all := l.viewUnlocked(mask)
	if all {
		return int(l.n - l.lowestUnlocked())
	}
	return len(clip(nums, l.lowestUnlocked()))
}

// ViewNums returns the record numbers of the display rows [offset, offset+count) of mask,
// newest first.
func (l *Log) ViewNums(mask Mask, offset, count int) []uint32 {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	lowest := l.lowestUnlocked()
	nums, all := l.viewUnlocked(mask)
	var total int
	if all {
		total = int(l.n - lowest)
	} else {
		nums = clip(nums, lowest)
		total = len(nums)
	}
	var out []uint32
	for i := offset; i < offset+count && i < total; i++ {
		if i < 0 {
			continue
		}
		pos := total - 1 - i
		if all {
			out = append(out, lowest+uint32(pos))
		} else {
			out = append(out, nums[pos])
		}
	}
	return out
}

// RecordsInMemory returns the records nums without touching the disk; ok is false when any of
// them is only in the spill file (use ReadRecords off the UI thread).
func (l *Log) RecordsInMemory(nums []uint32) (recs []Record, ok bool) {
	if l == nil {
		return nil, len(nums) == 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	recs = make([]Record, len(nums))
	for i, n := range nums {
		if n < l.memBase || n >= l.n {
			return nil, false
		}
		recs[i] = l.mem[n-l.memBase]
	}
	return recs, true
}

// ErrUnavailable reports records that are neither in memory nor in a readable spill file.
var ErrUnavailable = errors.New("oplog: record unavailable")

// ReadRecords returns the records nums, reading older ones from the spill file. It may block
// on disk IO: never call it from the UI thread.
func (l *Log) ReadRecords(nums []uint32) ([]Record, error) {
	if recs, ok := l.RecordsInMemory(nums); ok {
		return recs, nil
	}
	l.mu.Lock()
	if l.spilled {
		if err := l.w.Flush(); err != nil {
			l.breakDiskUnlocked()
		}
	}
	file, spilled, checkpoints, memBase, mem, n := l.file, l.spilled, l.checkpoints, l.memBase, l.mem, l.n
	l.mu.Unlock()

	recs := make([]Record, len(nums))
	var disk []int
	for i, num := range nums {
		switch {
		case num >= n:
			return nil, ErrUnavailable
		case num >= memBase:
			recs[i] = mem[num-memBase]
		case !spilled:
			return nil, ErrUnavailable
		default:
			disk = append(disk, i)
		}
	}
	if len(disk) == 0 {
		return recs, nil
	}
	slices.SortFunc(disk, func(a, b int) int { return int(nums[a]) - int(nums[b]) })
	rd := &fileReader{file: file, checkpoints: checkpoints}
	for _, i := range disk {
		rec, err := rd.at(nums[i])
		if err != nil {
			return nil, fmt.Errorf("oplog: read record %d: %w", nums[i], err)
		}
		recs[i] = rec
	}
	return recs, nil
}

// fileReader decodes records from a spill file, seeking by checkpoint and reading forward.
type fileReader struct {
	file        *os.File
	checkpoints []int64
	br          *bufio.Reader
	next        uint32 // record number br is positioned at
}

func (r *fileReader) at(num uint32) (Record, error) {
	block := num / checkpointEvery
	if r.br == nil || num < r.next || num/checkpointEvery > r.next/checkpointEvery {
		if int(block) >= len(r.checkpoints) {
			return Record{}, ErrUnavailable
		}
		r.br = bufio.NewReader(io.NewSectionReader(r.file, r.checkpoints[block], 1<<62))
		r.next = block * checkpointEvery
	}
	for {
		rec, err := decode(r.br)
		if err != nil {
			r.br = nil
			return Record{}, err
		}
		r.next++
		if r.next-1 == num {
			return rec, nil
		}
	}
}

func decode(br *bufio.Reader) (Record, error) {
	res, err := br.ReadByte()
	if err != nil {
		return Record{}, err
	}
	label, err := readString(br)
	if err != nil {
		return Record{}, err
	}
	reason, err := readString(br)
	if err != nil {
		return Record{}, err
	}
	return Record{Result: Result(res), Label: label, Reason: reason}, nil
}

func readString(br *bufio.Reader) (string, error) {
	n, err := binary.ReadUvarint(br)
	if err != nil {
		return "", err
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(br, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}
