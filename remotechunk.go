package ranger

import (
	"context"
	"io"

	"github.com/sourcegraph/conc/stream"
)

type RangedSource struct {
	byteRanges []ByteRange
	loader     Loader
	ranger     Ranger
	length     int64
}

func (rs RangedSource) ReadAt(p []byte, off int64) (n int, err error) {
	size := len(p)
	for n < size {
		offset := int64(n) + off
		chunkIndex := rs.ranger.Index(offset)
		chunk := rs.byteRanges[chunkIndex]
		chunkData, err := rs.loader.Load(chunk)
		if err != nil {
			return n, err
		}

		chunkOffset := offset % rs.ranger.chunkSize
		copied := copy(p[n:], chunkData[chunkOffset:])

		if copied == 0 {
			// We're finished, nothing left to copy
			return n, io.EOF
		}

		n += copied
	}
	return
}

type ReaderSeekerReadAt interface {
	io.Reader
	io.Seeker
	io.ReaderAt
	Size() int64
}

// Reader provides an io.Reader, io.Seeker and io.ReaderAt for the ranged source.
func (rs RangedSource) Reader() io.ReadSeeker {
	// the io.Reader, io.Seeker methods are stateful and need a
	// separate struct to track them. io.ReadAt is stateless and can be
	// implemented on main.
	return io.NewSectionReader(rs, 0, rs.length)
}

func (rs RangedSource) ReaderAt() io.ReaderAt {
	return rs
}

func (rs RangedSource) LookaheadReader(parallelism int) io.Reader {
	r, w := io.Pipe()
	ctx, cancel := context.WithCancel(context.TODO())

	go func() {
		defer w.Close()
		workStream := stream.New().WithMaxGoroutines(parallelism)
		for _, br := range rs.byteRanges {
			br := br
			select {
			case <-ctx.Done():
				break
			default:
			}
			workStream.Go(func() stream.Callback {
				data, err := rs.loader.Load(br)
				if err != nil {
					return func() {
						_ = w.CloseWithError(err)
						cancel()
					}
				}
				return func() {
					_, _ = w.Write(data)
				}
			})
		}
		workStream.Wait()
	}()

	return r
}

func (rs RangedSource) OffsetLookaheadReader(parallelism int, offset int64) io.Reader {
	allByteRanges := rs.byteRanges
	var relevantByteRanges []ByteRange
	for _, br := range allByteRanges {
		if br.To >= offset {
			relevantByteRanges = append(relevantByteRanges, br)
		}
	}

	r, w := io.Pipe()
	ctx, cancel := context.WithCancel(context.TODO())
	go func() {
		defer w.Close()
		workStream := stream.New().WithMaxGoroutines(parallelism)
		for _, br := range relevantByteRanges {
			br := br
			select {
			case <-ctx.Done():
				break
			default:
			}
			workStream.Go(func() stream.Callback {
				data, err := rs.loader.Load(br)
				if err != nil {
					return func() {
						_ = w.CloseWithError(err)
						cancel()
					}
				}
				dataOffset := int64(0)
				if br.From <= offset && offset <= br.To {
					dataOffset = offset - br.From
				}
				return func() {
					_, _ = w.Write(data[dataOffset:])
				}
			})
		}
		workStream.Wait()
	}()

	return r
}

func NewRangedSource(length int64, loader Loader, ranger Ranger) RangedSource {
	rf := RangedSource{
		byteRanges: ranger.Ranges(length),
		loader:     loader,
		ranger:     ranger,
		length:     length,
	}
	return rf
}
