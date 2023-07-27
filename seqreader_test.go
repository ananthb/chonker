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
