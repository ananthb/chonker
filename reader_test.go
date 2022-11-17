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

	chanReader := NewChannellingReader()

	go func() {
		chanReader.WriteFrom(one)
		chanReader.WriteFrom(two)
		chanReader.WriteFrom(three)
		chanReader.FinishWriting()
	}()
	bytes, _ := io.ReadAll(chanReader)
	assert.Equal(t, []byte("onetwothree"), bytes)
}
