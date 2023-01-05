package ranger

import (
	"fmt"
	"io"
)

type Chunk struct {
	Loader    Loader
	ByteRange ByteRange
}

func (c *Chunk) Load() ([]byte, error) {
	return c.Loader.Load(c.ByteRange)
}

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

func (r Ranger) Ranges(length int64) []ByteRange {
	ranges := make([]ByteRange, 0)
	for i := int64(0); i < length; i += r.chunkSize {
		br := ByteRange{
			From: i,
			To:   min(i+r.chunkSize-1, length-1),
		}
		ranges = append(ranges, br)
	}
	return ranges
}

func (r Ranger) Index(i int64) int {
	return int(i / r.chunkSize)
}

type RemoteFile struct {
	chunks []Chunk
	ranger Ranger
	length int64
}

func (r RemoteFile) ReadAt(p []byte, off int64) (n int, err error) {
	size := len(p)
	for n < size {
		offset := int64(n) + off
		chunkIndex := r.ranger.Index(offset)
		chunk := r.chunks[chunkIndex]
		chunkData, err := chunk.Load()
		if err != nil {
			return n, err
		}

		chunkOffset := offset % r.ranger.chunkSize
		copied := copy(p[n:], chunkData[chunkOffset:])

		if copied == 0 {
			// We're finished, nothing left to copy
			return n, io.EOF
		}

		n += copied
	}
	return
}

func (r RemoteFile) Reader() *io.SectionReader {
	return io.NewSectionReader(r, 0, r.length)
}

func NewRemoteFile(length int64, loader Loader, ranger Ranger) RemoteFile {
	chunks := make([]Chunk, 0)
	for _, br := range ranger.Ranges(length) {
		chunks = append(chunks, Chunk{
			Loader:    loader,
			ByteRange: br,
		})
	}
	rf := RemoteFile{
		chunks: chunks,
		ranger: ranger,
		length: length,
	}

	return rf
}
