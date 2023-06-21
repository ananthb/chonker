package ranger

import (
	"io"

	"github.com/sourcegraph/conc/pool"
)

type ParallelWriter struct {
	Length int64
	Loader Loader
	Ranger Ranger
}

func (pw ParallelWriter) WriteInto(w io.WriterAt, parallelism int) error {
	ranges := pw.Ranger.Ranges(pw.Length)
	p := pool.New().WithMaxGoroutines(parallelism).WithErrors()
	for _, r := range ranges {
		r := r
		p.Go(func() error {
			data, err := pw.Loader.Load(r)
			if err == nil {
				_, err = w.WriteAt(data, r.From)
			}
			return err
		})
	}
	return p.Wait()
}

func NewParallelWriter(length int64, loader Loader, ranger Ranger) *ParallelWriter {
	return &ParallelWriter{
		Length: length,
		Loader: loader,
		Ranger: ranger,
	}
}
