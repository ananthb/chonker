package ranger

import (
	"errors"
	"io"

	"github.com/sourcegraph/conc/stream"
)

// RangedSource represents a remote file that can be read in chunks using the given loader.
type RangedSource struct {
	loader Loader
	ranger Ranger
	length int64
}

type RemoteReader interface {
	io.Reader
	io.Seeker
	io.Closer
	io.ReaderAt
}

// Reader returns an io.Reader that reads the data in parallel, using
// a number of goroutines equal to the given parallelism count. Data is still
// returned in order. The rangedReadSeekCloser will start reading at the given offset.
func (rs RangedSource) Reader(parallelism int) RemoteReader {
	rrsc := &rangedReadSeekCloser{
		rs:          rs,
		parallelism: parallelism,
	}
	// don't init the reader here, because we want to be able to seek
	// without wasting loads.
	return rrsc
}

func (rs RangedSource) Ranges() []ByteRange {
	return rs.ranger.Ranges(rs.length)
}

type rangedReadSeekCloser struct {
	offset             int64
	rs                 RangedSource
	r                  *io.PipeReader
	parallelism        int
	cancellationSignal chan struct{}
}

func (rrsc *rangedReadSeekCloser) ReadAt(p []byte, off int64) (n int, err error) {
	clone := &rangedReadSeekCloser{
		rs:          rrsc.rs,
		parallelism: rrsc.parallelism,
	}
	_, err = clone.Seek(off, io.SeekStart)
	if err != nil {
		return 0, err
	}
	n, err = io.ReadFull(clone, p)
	if err == io.ErrUnexpectedEOF {
		err = io.EOF
	}
	return n, err

}

func (rrsc *rangedReadSeekCloser) Read(p []byte) (n int, err error) {
	if rrsc.r == nil {
		rrsc.init()
	}
	n, err = rrsc.r.Read(p)
	rrsc.offset += int64(n)
	return n, err
}

func (rrsc *rangedReadSeekCloser) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = rrsc.offset + offset
	case io.SeekEnd:
		newOffset = rrsc.rs.length + offset
	default:
		return 0, errors.New("invalid whence value")
	}

	if newOffset < 0 || newOffset > rrsc.rs.length {
		return 0, errors.New("seek out of bounds")
	}

	_ = rrsc.Close()
	rrsc.offset = newOffset

	return newOffset, nil
}

func (rrsc *rangedReadSeekCloser) Close() error {
	if rrsc.r != nil {
		_ = rrsc.r.Close()
	}
	rrsc.r = nil
	go func(sig chan struct{}) {
		sig <- struct{}{}
	}(rrsc.cancellationSignal)

	return nil
}

func (rrsc *rangedReadSeekCloser) init() {
	rrsc.cancellationSignal = make(chan struct{})
	var relevantByteRanges []ByteRange
	for _, br := range rrsc.rs.Ranges() {
		if br.To >= rrsc.offset {
			relevantByteRanges = append(relevantByteRanges, br)
		}
	}

	r, w := io.Pipe()
	rrsc.r = r

	go func(offset int64, cancelSig chan struct{}) {
		defer w.Close()
		workStream := stream.New().WithMaxGoroutines(rrsc.parallelism)
		for _, br := range relevantByteRanges {
			br := br
			workStream.Go(func() stream.Callback {
				select {
				case <-cancelSig:
					return func() {}
				default:
					data, err := rrsc.rs.loader.Load(br)
					if err != nil {
						return func() {
							_ = w.CloseWithError(err)
						}
					}
					chunkOffset := int64(0)
					if br.Contains(offset) {
						chunkOffset = offset - br.From
					}
					return func() {
						_, err = w.Write(data[chunkOffset:])
						if err != nil {
							_ = w.CloseWithError(err)
						}
					}
				}

			})
		}
		workStream.Wait()
	}(rrsc.offset, rrsc.cancellationSignal)
}

func NewRangedSource(length int64, loader Loader, ranger Ranger) RangedSource {
	rf := RangedSource{
		loader: loader,
		ranger: ranger,
		length: length,
	}
	return rf
}
