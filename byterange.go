package ranger

import "fmt"

// ByteRange represents a range of bytes available in a file
type ByteRange struct {
	From int64
	To   int64
}

// RangeHeader returns the HTTP header representation of the byte range,
// suitable for use in the Range header, as described in https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Range
func (r ByteRange) RangeHeader() string {
	return fmt.Sprintf("bytes=%v-%v", r.From, r.To)
}

// Length returns the length of the byte range.
func (r ByteRange) Length() int64 {
	return r.To - r.From + 1
}
