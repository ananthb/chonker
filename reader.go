package ranger

import (
	"io"
)

type ChannelReader struct {
	inputs chan io.Reader
	r      io.Reader
	w      io.Writer
	io.Reader
}

func NewChannelReader() *ChannelReader {
	r, w := io.Pipe()
	cr := &ChannelReader{
		inputs: make(chan io.Reader),
		r:      r,
		w:      w,
	}
	go cr.Run()
	return cr
}

func (cr *ChannelReader) Run() {
	for currentReader := range cr.inputs {
		_, err := io.Copy(cr.w, currentReader)
		panicIfErr(err) // TODO switch to context?
		if rc, ok := currentReader.(io.ReadCloser); ok {
			_ = rc.Close()
		}

	}
	if wc, ok := cr.w.(io.WriteCloser); ok {
		_ = wc.Close()
	}
}

func (cr *ChannelReader) Read(p []byte) (n int, err error) {
	return cr.r.Read(p)
}

func (cr *ChannelReader) Inputs() chan<- io.Reader {
	return cr.inputs
}

func (cr *ChannelReader) Close() {
	close(cr.inputs)
}
