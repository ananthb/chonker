package ranger

import (
	"io"
)

type ChannellingReader struct {
	ch chan<- io.Reader
	r  io.Reader
	w  io.Writer
	io.ReadCloser
}

func NewChannellingReader() *ChannellingReader {
	r, w := io.Pipe()
	ch := make(chan io.Reader)
	go func() {
		for rCurrent := range ch {
			_, err := io.Copy(w, rCurrent)
			panicIfErr(err)
			if rc, ok := rCurrent.(io.ReadCloser); ok {
				err = rc.Close()
				panicIfErr(err)
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

func (cr *ChannellingReader) FinishWriting() {
	close(cr.ch)
}

func (cr *ChannellingReader) WriteFrom(r io.Reader) {
	cr.ch <- r
}

func (cr *ChannellingReader) Close() error {
	return nil
}
