package ranger

import (
	"fmt"
	"io"

	"github.com/sudhirj/cirque"
)

type ByteRange struct {
	From int64
	To   int64
}

func (r ByteRange) Header() string {
	return fmt.Sprintf("bytes=%v-%v", r.From, r.To)
}

func (r ByteRange) Length() int64 {
	return r.To - r.From + 1
}

type Ranger struct {
	chunkSize int64
}

func NewRanger(chunkSize int64) Ranger {
	return Ranger{chunkSize: chunkSize}
}

func (r Ranger) Ranges(length int64, offset int64) []ByteRange {
	ranges := make([]ByteRange, 0)
	for runningOffset := int64(0); runningOffset < length; runningOffset += r.chunkSize {
		br := ByteRange{
			From: runningOffset,
			To:   min(runningOffset+r.chunkSize-1, length-1),
		}
		if offset > br.To {
			continue
		}
		if br.From < offset {
			br.From = offset
		}
		ranges = append(ranges, br)
	}
	return ranges
}

type errorReader struct {
	err error
}

func (e errorReader) Read([]byte) (n int, err error) {
	return 0, e.err
}

func (r Ranger) RangedReader(length int64, offset int64, loader func(br ByteRange) (io.Reader, error), parallelism int) io.Reader {
	cr := NewChannelReader()

	// use cirque to manage an ordered worker pool (order is important because we want
	// the readers to come out in byte range order, or we'll jumble the data).
	inputRanges, outputReaders := cirque.NewCirque(int64(parallelism), func(br ByteRange) io.Reader {
		r, err := loader(br)
		if err != nil {
			return errorReader{err: err}
		}
		return r
	})

	// send byte Ranges as input to the worker pool
	go func() {
		for _, br := range r.Ranges(length, offset) {
			inputRanges <- br
		}
		close(inputRanges)
	}()

	// add the worker pool outputs to the Reader
	go func() {
		for r := range outputReaders {
			cr.Inputs() <- r
		}
		cr.Close()
	}()

	return cr
}
