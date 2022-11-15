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

	chanReader := NewChannellingReader(0)

	go func() {
		chanReader.Send(one)
		chanReader.Send(two)
		chanReader.Send(three)
		chanReader.Finish()
	}()
	bytes, _ := io.ReadAll(chanReader)
	assert.Equal(t, []byte("onetwothree"), bytes)
}
