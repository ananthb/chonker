package ranger

import "fmt"

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
