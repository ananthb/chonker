package ranger

import (
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"testing"
)

func TestSeqReader(t *testing.T) {
	content := makeData(10)
	server := makeHTTPServer(t, content)

	ranger := NewSizedRanger(int64(len(content)), NewRanger(2))
	sr := NewSeqReader(http.DefaultClient, server.URL, ranger)

	received, err := io.ReadAll(sr)

	assert.NoError(t, err)
	assert.Equal(t, content, received)
}

func TestSeqReaderWithOffset(t *testing.T) {
	content := makeData(10)
	server := makeHTTPServer(t, content)

	ranger := NewSizedRanger(int64(len(content)), NewRanger(2))
	sr := NewSeqReader(http.DefaultClient, server.URL, ranger)

	tests := []struct {
		offset       int64
		whence       int
		expectedSeek int64
		expected     []byte
	}{
		{3, io.SeekCurrent, 3, content[3:]},
		{5, io.SeekStart, 5, content[5:]},
		{0, io.SeekStart, 0, content[:]},
		{6, io.SeekStart, 6, content[6:]},
		{-2, io.SeekEnd, 8, content[8:]},
	}

	for _, test := range tests {
		seek, err := sr.Seek(test.offset, test.whence)
		assert.NoError(t, err, test)
		assert.Equal(t, test.expectedSeek, seek, test)

		received, err := io.ReadAll(sr)

		assert.NoError(t, err, test)
		assert.Equal(t, test.expected, received, test)
	}
}
