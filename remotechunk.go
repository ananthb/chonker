package ranger

import (
	"io"
)

type RangedSource struct {
	chunks []Chunk
	ranger Ranger
	length int64
}

func (rs RangedSource) ReadAt(p []byte, off int64) (n int, err error) {
	size := len(p)
	for n < size {
		offset := int64(n) + off
		chunkIndex := rs.ranger.Index(offset)
		chunk := rs.chunks[chunkIndex]
		chunkData, err := chunk.Load()
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

func NewRangedSource(length int64, loader Loader, ranger Ranger) RangedSource {
	chunks := make([]Chunk, 0)
	for _, br := range ranger.Ranges(length) {
		chunks = append(chunks, Chunk{
			Loader:    loader,
			ByteRange: br,
		})
	}
	rf := RangedSource{
		chunks: chunks,
		ranger: ranger,
		length: length,
	}

	return rf
}
