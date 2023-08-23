package ranger

import (
	"fmt"
	"net/http"
)

// ByteRange represents a range of bytes available in a file
type ByteRange struct {
	From int64
	To   int64
}

// RangeHeader returns the HTTP header representation of the byte range,
// suitable for use in the Range header, as described in https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Range
func (br ByteRange) RangeHeader() string {
	return fmt.Sprintf("bytes=%v-%v", br.From, br.To)
}

// Length returns the length of the byte range.
func (br ByteRange) Length() int64 {
	return br.To - br.From + 1
}

func (br ByteRange) Contains(offset int64) bool {
	return br.From <= offset && offset <= br.To
}

func (br ByteRange) Floor(offset int64) ByteRange {
	return ByteRange{
		From: max(br.From, offset),
		To:   br.To,
	}
}

func (br ByteRange) Request(url string) (req *http.Request, err error) {
	req, err = http.NewRequest(http.MethodGet, url, nil)
	if err == nil {
		req.Header.Set("Range", br.RangeHeader())
	}
	return
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
