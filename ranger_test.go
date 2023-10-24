package ranger

import (
	"bytes"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRangingTransport(t *testing.T) {
	content := makeData(100)
	server := makeHTTPServer(t, content)

	clients := []*http.Client{
		http.DefaultClient, // all clients must behave the same as the default HTTP client
		{Transport: NewRoundTripper(nil, 10, 1)},
		{Transport: NewRoundTripper(nil, 1000, 10)},
	}
	testCases := []struct {
		rangeHeader string
		expected    []byte
	}{
		{expected: content},
		{rangeHeader: "bytes=42-", expected: content[42:]},
		{rangeHeader: "bytes=42-84", expected: content[42:85]},
	}

	for clientIndex, client := range clients {
		for _, testCase := range testCases {
			t.Run(
				"Client["+strconv.Itoa(clientIndex)+"]:"+testCase.rangeHeader,
				func(t *testing.T) {
					req, err := http.NewRequest(http.MethodGet, server.URL, nil)
					assert.NoError(t, err)
					if testCase.rangeHeader != "" {
						req.Header.Set("Range", testCase.rangeHeader)
					}
					response, err := client.Do(req)
					assert.NoError(t, err)
					defer response.Body.Close()
					servedContent, err := io.ReadAll(response.Body)
					assert.NoError(t, err)
					assert.Equal(t, testCase.expected, servedContent)
				},
			)
		}
	}
}

func makeData(size int) []byte {
	rnd := rand.New(rand.NewSource(42))
	content := make([]byte, size)
	rnd.Read(content)
	return content
}

func makeHTTPServer(t *testing.T, content []byte) *httptest.Server {
	server := httptest.NewServer(
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			http.ServeContent(writer, request, "", time.Now(), bytes.NewReader(content))
		}),
	)
	return server
}
