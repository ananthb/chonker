package ranger

import (
	"net/http"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
)

func TestRemoteFileReader(t *testing.T) {
	content := makeData(1000)
	server := makeHTTPServer(t, content)

	for _, size := range []int64{1, 3, 512, 1024, 2048} {
		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		resp, err := Do(nil, req, size, 10)
		assert.NoError(t, err)
		assert.NoError(t, iotest.TestReader(resp.Body, content))
		resp.Body.Close()
	}
}
