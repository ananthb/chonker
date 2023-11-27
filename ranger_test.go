package ranger

import (
	"bytes"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDo(t *testing.T) {
	content := makeData(1024 * 10)
	server := makeHTTPServer(t, content)

	testCases := []struct {
		name        string
		rangeHeader string
		chunkSize   int64
		bufNum      int64
		expected    []byte
		err         bool
	}{
		{
			name:        "Start at 42",
			rangeHeader: "bytes=42-",
			chunkSize:   1024,
			bufNum:      100,
			expected:    content[42:],
		},
		{name: "1 byte chunk", chunkSize: 1, bufNum: 1, expected: content},
		{name: "3KiB chunks", chunkSize: 3 * 1024, bufNum: 5, expected: content},
		{name: "2KiB chunks", chunkSize: 2048, bufNum: 8, expected: content},
		{name: "16MiB chunks", chunkSize: 16 * 1024 * 1024, bufNum: 100, expected: content},
		{name: "single chunk buffer", chunkSize: 1024, bufNum: 1, expected: content},
		{name: "0 chunk size", bufNum: 100, err: true},
		{name: "0 buffer number", chunkSize: 1024, err: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, server.URL, nil)
			assert.NoError(t, err)
			if testCase.rangeHeader != "" {
				req.Header.Set("Range", testCase.rangeHeader)
			}
			resp, err := Do(nil, req, testCase.chunkSize, testCase.bufNum)
			if testCase.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				defer resp.Body.Close()
				assert.NoError(t, iotest.TestReader(resp.Body, testCase.expected))
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	content := makeData(1024 * 10)
	server := makeHTTPServer(t, content)
	client := NewClient(nil, 1024, 100)
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	assert.NoError(t, err)
	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.NoError(t, iotest.TestReader(resp.Body, content))
}

func TestNewRoundTripper(t *testing.T) {
	content := makeData(1024 * 10)
	server := makeHTTPServer(t, content)
	transport := NewRoundTripper(nil, 1024, 100)
	client := &http.Client{Transport: transport}
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	assert.NoError(t, err)
	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.NoError(t, iotest.TestReader(resp.Body, content))
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
