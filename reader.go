package ranger

import (
	"io"
)

type ChannelReader struct {
	inputs chan io.Reader
	r      io.Reader
	w      io.Writer
	curr   io.Reader
	io.Reader
}

func NewChannelReader() *ChannelReader {
	r, w := io.Pipe()
	cr := &ChannelReader{
		inputs: make(chan io.Reader),
		r:      r,
		w:      w,
		curr:   nil,
	}
	return cr
}

func (cr *ChannelReader) Read(p []byte) (n int, err error) {
	if cr.curr == nil {
		// we're at the beginning (or the end)
		cr.curr = <-cr.inputs
	}
	if cr.curr == nil {
		// still nil? we're done, input channel was closed
		return 0, io.EOF
	}

	n, err = cr.curr.Read(p)

	if err == io.EOF {
		// this reader is done, let's move to the next one
		cr.curr = <-cr.inputs
		return n, nil
	}
	return
}

func (cr *ChannelReader) Inputs() chan<- io.Reader {
	return cr.inputs
}

func (cr *ChannelReader) Close() {
	close(cr.inputs)
}
