package ranger

import (
	"fmt"
	"golang.org/x/exp/constraints"
)

type byteRange struct {
	from int64
	to   int64
}

func (r byteRange) header() string {
	return fmt.Sprintf("bytes=%v-%v", r.from, r.to)
}

type Ranger struct {
	offset    int64
	length    int64
	chunkSize int64
}

func (r Ranger) ranges() []byteRange {
	ranges := make([]byteRange, 0)
	for runningOffset := r.offset; runningOffset < r.length; runningOffset += r.chunkSize {
		ranges = append(ranges, byteRange{
			from: runningOffset,
			to:   min(runningOffset+r.chunkSize-1, r.length-1),
		})
	}
	return ranges
}

func min[T constraints.Ordered](a T, b T) T {
	if a < b {
		return a
	}
	return b
}

func NewRanger(offset int64, length int64, chunkSize int64) Ranger {
	return Ranger{offset: offset, length: length, chunkSize: chunkSize}
}
