package ranger

import (
	"io"
)

type ChannellingReader struct {
	ch chan<- io.Reader
	r  io.Reader
	w  io.Writer
}

func NewChannellingReader(bufferSize int) *ChannellingReader {
	r, w := io.Pipe()
	ch := make(chan io.Reader, bufferSize)
	go func() {
		for rCurrent := range ch {
			_, _ = io.Copy(w, rCurrent)
			if rc, ok := rCurrent.(io.ReadCloser); ok {
				_ = rc.Close()
			}
		}
		_ = w.Close()
	}()
	return &ChannellingReader{
		ch: ch,
		r:  r,
		w:  w,
	}
}

func (cr *ChannellingReader) Read(p []byte) (n int, err error) {
	return cr.r.Read(p)
}

func (cr *ChannellingReader) Close() {
	close(cr.ch)
}

func (cr *ChannellingReader) Send(r io.Reader) {
	cr.ch <- r
}
