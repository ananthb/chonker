package ranger

import (
	"fmt"
)

type byteRange struct {
	from int64
	to   int64
}

func (r byteRange) Header() string {
	return fmt.Sprintf("bytes=%v-%v", r.from, r.to)
}

type Ranger struct {
	chunkSize int64
}

func (r Ranger) Ranges(length int64, offset int64) []byteRange {
	ranges := make([]byteRange, 0)
	for runningOffset := int64(0); runningOffset < length; runningOffset += r.chunkSize {
		br := byteRange{
			from: runningOffset,
			to:   min(runningOffset+r.chunkSize-1, length-1),
		}
		if offset > br.to {
			continue
		}
		if br.from < offset {
			br.from = offset
		}
		ranges = append(ranges, br)
	}
	return ranges
}

func NewRanger(chunkSize int64) Ranger {
	return Ranger{chunkSize: chunkSize}
}
