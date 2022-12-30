package ranger

import (
	"bytes"
	"fmt"
	"io"
)

// Loader implements a Load method that provides data as an io.Reader for a
// given byte range chunk. Load should be safe to call from multiple goroutines.
// If the Loader is being used with the Preload option, it should be safe to
// call the Load function multiple times in quick succession for the same input - using a caching
// system like https://github.com/golang/groupcache would make a lot of sense in that case.
type Loader interface {
	Load(br ByteRange) ([]byte, error)
}
type LoaderFunc func(br ByteRange) ([]byte, error)

func (l LoaderFunc) Load(br ByteRange) ([]byte, error) {
	return l(br)
}

type ChunkAlignedRemoteFile struct {
	Loader        Loader
	Length        int64
	Ranger        Ranger
	currentReader io.Reader
}

func (rf *ChunkAlignedRemoteFile) Read(p []byte) (n int, err error) {
	if rf.currentReader == nil {
		rf.currentReader = io.MultiReader(rf.Readers()...)
	}
	return rf.currentReader.Read(p)
}

func (rf *ChunkAlignedRemoteFile) Readers() []io.Reader {
	chunks := rf.Chunks()
	readers := make([]io.Reader, len(chunks))
	for i := range chunks {
		readers[i] = &chunks[i]
	}
	return readers
}

func (rf *ChunkAlignedRemoteFile) Chunks() []Chunk {
	ranges := rf.Ranger.Ranges(rf.Length, 0)
	chunks := make([]Chunk, len(ranges))
	for i, br := range ranges {
		chunks[i] = Chunk{
			RemoteFile: rf,
			ByteRange:  br,
		}
	}
	return chunks
}

type Chunk struct {
	RemoteFile *ChunkAlignedRemoteFile
	ByteRange  ByteRange
	reader     io.Reader
}

func (c *Chunk) Close() error {
	if c.reader != nil {
		if closer, ok := c.reader.(io.Closer); ok {
			return closer.Close()
		}
	}
	return nil
}

func (c *Chunk) Read(p []byte) (n int, err error) {
	if c.reader == nil {
		data, err := c.Load()
		if err != nil {
			return 0, err
		}
		c.reader = bytes.NewReader(data)
	}
	return c.reader.Read(p)
}

func (c *Chunk) Load() ([]byte, error) {
	return c.RemoteFile.Loader.Load(c.ByteRange)
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

func NewChunkAlignedRemoteFile(length int64, loader Loader, ranger Ranger) *ChunkAlignedRemoteFile {
	return &ChunkAlignedRemoteFile{
		Loader: loader,
		Length: length,
		Ranger: ranger,
	}
}
