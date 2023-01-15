package ranger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestByteRangeHeader(t *testing.T) {
	// Needs to use the format `bytes=0-50`. See https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Range
	ranger := NewRanger(10)
	ranges := ranger.Ranges(100)
	assert.Equal(t, ByteRange{From: 0, To: 9}, ranges[0])
	assert.Equal(t, "bytes=0-9", ranges[0].RangeHeader())
	assert.Equal(t, ByteRange{From: 10, To: 19}, ranges[1])
	assert.Equal(t, "bytes=10-19", ranges[1].RangeHeader())
}

func TestByteRangeLength(t *testing.T) {
	assert.Equal(t, int64(10), ByteRange{From: 0, To: 9}.Length())
	assert.Equal(t, int64(1), ByteRange{From: 0, To: 0}.Length())
}
