package ranger

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasicChanneledReading(t *testing.T) {
	one := strings.NewReader("one")
	two := strings.NewReader("two")
	three := strings.NewReader("three")

	chanReader := NewChannelReader()

	go func() {
		var r io.Reader = one
		chanReader.inputs <- r
		var r2 io.Reader = two
		chanReader.inputs <- r2
		var r3 io.Reader = three
		chanReader.inputs <- r3
		close(chanReader.inputs)
	}()
	bytes, _ := io.ReadAll(chanReader)
	assert.Equal(t, []byte("onetwothree"), bytes)
}
