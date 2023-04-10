package ranger

import (
	"context"
	"io"

	"github.com/sourcegraph/conc/stream"
)

// RangedSource represents a remote file that can be read in chunks using the given loader.
type RangedSource struct {
	cachedByteRanges []ByteRange
	loader           Loader
	ranger           Ranger
	length           int64
}

// ParallelReader returns an io.Reader that reads the data in parallel, using
// a number of goroutines equal to the given parallelism count. Data is still
// returned in order.
func (rs RangedSource) ParallelReader(ctx context.Context, parallelism int) io.Reader {
	return rs.ParallelOffsetReader(ctx, parallelism, 0)
}

// ParallelOffsetReader returns an io.Reader that reads the data in parallel, using
// a number of goroutines equal to the given parallelism count. Data is still
// returned in order. The reader will start reading at the given offset.
func (rs RangedSource) ParallelOffsetReader(ctx context.Context, parallelism int, offset int64) io.Reader {
	var relevantByteRanges []ByteRange
	for _, br := range rs.cachedByteRanges {
		if br.To >= offset {
			relevantByteRanges = append(relevantByteRanges, br)
		}
	}

	r, w := io.Pipe()
	ctx, cancel := context.WithCancel(ctx)
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
				if br.Contains(offset) {
					dataOffset = offset - br.From
				}
				return func() {
					_, err = w.Write(data[dataOffset:])
					if err != nil {
						cancel()
					}
				}
			})
		}
		workStream.Wait()
	}()

	return r
}

func NewRangedSource(length int64, loader Loader, ranger Ranger) RangedSource {
	rf := RangedSource{
		cachedByteRanges: ranger.Ranges(length),
		loader:           loader,
		ranger:           ranger,
		length:           length,
	}
	return rf
}
