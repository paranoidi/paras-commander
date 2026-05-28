package dialog

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
)

const pathPickerMinPathWidth = 8

type pathPickerFieldRange struct {
	text  string
	start int // rune offset in SearchLine
	end   int
}

func (i PathPickerItem) searchFieldRanges() (source, name, path pathPickerFieldRange) {
	pos := 0
	if i.Source != "" {
		n := utf8.RuneCountInString(i.Source)
		source = pathPickerFieldRange{text: i.Source, start: pos, end: pos + n}
		pos = source.end + 1
	}
	if i.Name != "" {
		n := utf8.RuneCountInString(i.Name)
		name = pathPickerFieldRange{text: i.Name, start: pos, end: pos + n}
		pos = name.end + 1
	}
	if i.Path != "" {
		n := utf8.RuneCountInString(i.Path)
		path = pathPickerFieldRange{text: i.Path, start: pos, end: pos + n}
	}
	return source, name, path
}

func pathPickerMaxRuneWidth(items []PathPickerItem, field func(PathPickerItem) string) int {
	max := 0
	for _, item := range items {
		if w := utf8.RuneCountInString(field(item)); w > max {
			max = w
		}
	}
	return max
}

func pathPickerSeparatorCount(nameW int) int {
	if nameW > 0 {
		return 2
	}
	return 1
}

func pathPickerColumnWidths(items []PathPickerItem, rowWidth int) (sourceW, nameW, pathW int) {
	if rowWidth <= 0 {
		return 0, 0, 0
	}
	sourceW = pathPickerMaxRuneWidth(items, func(i PathPickerItem) string { return i.Source })
	nameW = pathPickerMaxRuneWidth(items, func(i PathPickerItem) string { return i.Name })
	pathW = rowWidth - sourceW - nameW - pathPickerSeparatorCount(nameW)
	for pathW < pathPickerMinPathWidth {
		shrunk := false
		if nameW > 0 {
			nameW--
			shrunk = true
		} else if sourceW > 1 {
			sourceW--
			shrunk = true
		} else if sourceW > 0 && pathW < 1 {
			sourceW = 0
			shrunk = true
		}
		if !shrunk {
			break
		}
		pathW = rowWidth - sourceW - nameW - pathPickerSeparatorCount(nameW)
	}
	if pathW < 1 {
		pathW = 1
	}
	return sourceW, nameW, pathW
}

func pathPickerPadField(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if utf8.RuneCountInString(value) <= width {
		return value + strings.Repeat(" ", width-utf8.RuneCountInString(value))
	}
	return primitive.TruncateRight(value, width)
}

func pathPickerRowDisplay(item PathPickerItem, sourceW, nameW, pathW int) string {
	if item.Empty() {
		return ""
	}
	var b strings.Builder
	b.WriteString(pathPickerPadField(item.Source, sourceW))
	b.WriteByte(' ')
	if nameW > 0 {
		b.WriteString(pathPickerPadField(item.Name, nameW))
		b.WriteByte(' ')
	}
	if item.Path != "" {
		b.WriteString(primitive.FitPathForWidth(item.Path, pathW))
	}
	return b.String()
}

func pathPickerIntersectRange(a, b search.Range) search.Range {
	start := a.Start
	if b.Start > start {
		start = b.Start
	}
	end := a.End
	if b.End < end {
		end = b.End
	}
	if end < start {
		return search.Range{}
	}
	return search.Range{Start: start, End: end}
}

func pathPickerFieldHighlightSpans(
	ranges []search.Range,
	field pathPickerFieldRange,
	disp string,
	dispOffset int,
	pathFit bool,
	matchStyle tcell.Style,
) []primitive.Span {
	if field.end <= field.start || disp == "" {
		return nil
	}
	var fieldRanges []search.Range
	for _, r := range ranges {
		overlap := pathPickerIntersectRange(r, search.Range{Start: field.start, End: field.end})
		if overlap.End <= overlap.Start {
			continue
		}
		fieldRanges = append(fieldRanges, search.Range{
			Start: overlap.Start - field.start,
			End:   overlap.End - field.start,
		})
	}
	if len(fieldRanges) == 0 {
		return nil
	}
	dispWidth := utf8.RuneCountInString(disp)
	var spans []primitive.Span
	if pathFit {
		_, spans = fuzzyPathRowContent(field.text, fieldRanges, dispWidth, matchStyle)
	} else {
		_, spans = fuzzyRowContent(field.text, fieldRanges, dispWidth, matchStyle, false)
	}
	for i := range spans {
		spans[i].Start += dispOffset
		spans[i].End += dispOffset
	}
	return spans
}

func pathPickerHighlightSpans(
	ranges []search.Range,
	item PathPickerItem,
	sourceW, nameW, pathW int,
	matchStyle tcell.Style,
) []primitive.Span {
	if len(ranges) == 0 {
		return nil
	}
	source, name, path := item.searchFieldRanges()
	sourceDisp := pathPickerPadField(item.Source, sourceW)
	offset := utf8.RuneCountInString(sourceDisp) + 1

	var spans []primitive.Span
	spans = append(spans, pathPickerFieldHighlightSpans(ranges, source, sourceDisp, 0, false, matchStyle)...)

	if nameW > 0 {
		nameDisp := pathPickerPadField(item.Name, nameW)
		spans = append(spans, pathPickerFieldHighlightSpans(ranges, name, nameDisp, offset, false, matchStyle)...)
		offset += utf8.RuneCountInString(nameDisp) + 1
	}

	pathDisp := primitive.FitPathForWidth(item.Path, pathW)
	spans = append(spans, pathPickerFieldHighlightSpans(ranges, path, pathDisp, offset, true, matchStyle)...)
	return spans
}

func pathPickerRowContent(
	item PathPickerItem,
	ranges []search.Range,
	sourceW, nameW, pathW int,
	matchStyle tcell.Style,
) (string, []primitive.Span) {
	if item.Empty() {
		return "", nil
	}
	rowWidth := sourceW + nameW + pathW + pathPickerSeparatorCount(nameW)
	if rowWidth <= 0 {
		return "", nil
	}
	text := pathPickerRowDisplay(item, sourceW, nameW, pathW)
	spans := pathPickerHighlightSpans(ranges, item, sourceW, nameW, pathW, matchStyle)
	return text, spans
}
