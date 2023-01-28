package ranger

import (
	"io"
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

func NewRangedSource(length int64, loader Loader, ranger Ranger) RangedSource {
	rf := RangedSource{
		byteRanges: ranger.Ranges(length),
		loader:     loader,
		ranger:     ranger,
		length:     length,
	}
	return rf
}
