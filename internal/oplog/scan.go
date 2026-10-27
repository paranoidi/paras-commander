package oplog

import (
	"bufio"
	"context"
	"io"

	"github.com/paranoidi/paras-commander/internal/search"
)

// scanBatch is how many records Scan examines between onBatch calls and cancel checks.
const scanBatch = 16384

// Scan finds the records from..n-1 of source (NoSource for all) whose result is in mask and
// whose label matches query, calling onBatch with each batch of new matches (ascending record
// numbers) as it goes. It returns the record number it scanned up to, so a later Scan can
// continue from there for records appended meanwhile. It reads the spill file sequentially and may block on disk IO:
// never call it from the UI thread.
func (l *Log) Scan(ctx context.Context, from uint32, mask Mask, source int, query search.Query, opts search.Options, onBatch func([]uint32)) (uint32, error) {
	l.mu.Lock()
	if l.spilled {
		if err := l.w.Flush(); err != nil {
			l.breakDiskUnlocked()
		}
	}
	file, spilled, memBase, n, checkpoints := l.file, l.spilled, l.memBase, l.n, l.checkpoints
	mem := l.mem
	lowest := l.lowestUnlocked()
	superseded := make(map[uint32]struct{}, len(l.superseded))
	for k := range l.superseded {
		superseded[k] = struct{}{}
	}
	l.mu.Unlock()

	from = max(from, lowest)
	var batch []uint32
	examined := 0
	check := func(num uint32, rec Record) error {
		_, hidden := superseded[num]
		inSource := source == NoSource || int(rec.Source) == source
		if !hidden && inSource && mask.Has(rec.Result) && query.Match(rec.Label, opts).Matched {
			batch = append(batch, num)
		}
		examined++
		if examined%scanBatch == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
			if len(batch) > 0 {
				onBatch(batch)
				batch = nil
			}
		}
		return nil
	}

	if spilled && from < memBase {
		block := min(int(from/checkpointEvery), len(checkpoints)-1)
		br := bufio.NewReaderSize(io.NewSectionReader(file, checkpoints[block], 1<<62), writeBufferBytes)
		for num := uint32(block) * checkpointEvery; num < memBase; num++ {
			rec, err := decode(br)
			if err != nil {
				return num, err
			}
			if num < from {
				continue
			}
			if err := check(num, rec); err != nil {
				return num, err
			}
		}
	}
	for num := max(from, memBase); num < n; num++ {
		if err := check(num, mem[num-memBase]); err != nil {
			return num, err
		}
	}
	if len(batch) > 0 {
		onBatch(batch)
	}
	return n, ctx.Err()
}
