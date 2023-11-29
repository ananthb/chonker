package ranger

import (
	"bytes"
	"context"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewRequestWithContext(t *testing.T) {
	type expected struct {
		err bool
		req *Request
	}
	testCases := []struct {
		name     string
		chunk    int64
		workers  int64
		expected expected
	}{
		{
			name:    "zero chunk",
			chunk:   0,
			workers: 1,
			expected: expected{
				err: true,
			},
		},
		{
			name:    "zero workers",
			chunk:   1,
			workers: 0,
			expected: expected{
				err: true,
			},
		},
		{
			name:    "valid",
			chunk:   1,
			workers: 1,
			expected: expected{
				req: &Request{
					chunkSize: 1,
					workers:   1,
				},
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			req, err := NewRequestWithContext(
				ctx,
				http.MethodGet,
				"http://example.com",
				nil,
				testCase.chunk,
				testCase.workers,
			)
			if testCase.expected.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expected.req.chunkSize, req.chunkSize)
				assert.Equal(t, testCase.expected.req.workers, req.workers)
			}
		})
	}
}

func TestDo(t *testing.T) {
	content := makeData(10 * 1024) // Default content. 10KiB.

	type expected struct {
		body               []byte
		contentLength      int64
		contentRangeHeader string
	}

	testCases := []struct {
		name        string
		content     []byte
		rangeHeader string
		request     Request
		expected    expected
		timeout     time.Duration
		err         bool
	}{
		{
			name:        "Start at 42",
			rangeHeader: "bytes=42-",
			request: Request{
				chunkSize: 1024,
				workers:   100,
			},
			expected: expected{
				body:               content[42:],
				contentLength:      int64(len(content[42:])),
				contentRangeHeader: "bytes 42-10239/10240",
			},
		},
		{
			name:        "small range",
			rangeHeader: "bytes=42-83",
			request: Request{
				chunkSize: 5,
				workers:   10,
			},
			expected: expected{
				body:               content[42:84],
				contentLength:      int64(len(content[42:84])),
				contentRangeHeader: "bytes 42-83/10240",
			},
		},
		{
			name:        "error fetching multiple ranges",
			rangeHeader: "bytes=100-200,300-400",
			err:         true,
		},
		{
			name: "1 byte chunk",
			request: Request{
				chunkSize: 1,
				workers:   1,
			},
			expected: expected{body: content, contentLength: int64(len(content))},
		},
		{
			name: "3KiB chunks",
			request: Request{
				chunkSize: 3 * 1024,
				workers:   5,
			},
			expected: expected{body: content, contentLength: int64(len(content))},
		},
		{
			name: "2KiB chunks",
			request: Request{
				chunkSize: 2048,
				workers:   8,
			},
			expected: expected{body: content, contentLength: int64(len(content))},
		},
		{
			name: "16MiB chunks",
			request: Request{
				chunkSize: 16 * 1024 * 1024,
				workers:   100,
			},
			expected: expected{body: content, contentLength: int64(len(content))},
		},
		{
			name: "single chunk buffer",
			request: Request{
				chunkSize: 1024,
				workers:   1,
			},
			expected: expected{body: content, contentLength: int64(len(content))},
		},
		{
			name:        "invalid range",
			rangeHeader: "bytes=100-50",
			err:         true,
		},
		{
			name: "test timeout",
			request: Request{
				chunkSize: 1,
				workers:   1,
			},
			content: makeData(1024 * 1024), // 1MiB
			timeout: 1 * time.Millisecond,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.content == nil {
				testCase.content = content
			}
			server := makeHTTPServer(content)
			defer server.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if testCase.timeout > 0 {
				ctx, cancel = context.WithTimeout(ctx, testCase.timeout)
				defer cancel()
			}

			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodGet,
				server.URL,
				nil,
			)
			assert.NoError(t, err)

			if testCase.rangeHeader != "" {
				req.Header.Set("Range", testCase.rangeHeader)
			}
			testCase.request.Request = req

			resp, err := Do(nil, &testCase.request)
			if testCase.timeout == 0 {
				if testCase.err {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					defer resp.Body.Close()
					assert.Equal(t, testCase.expected.contentLength, resp.ContentLength)
					assert.Equal(t, testCase.expected.contentRangeHeader, resp.Header.Get("Content-Range"))
					assert.NoError(t, iotest.TestReader(resp.Body, testCase.expected.body))
				}
			} else {
				<-ctx.Done()
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	content := makeData(1024 * 10)
	server := makeHTTPServer(content)
	defer server.Close()

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
	server := makeHTTPServer(content)
	defer server.Close()

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

func makeHTTPServer(content []byte) *httptest.Server {
	server := httptest.NewServer(
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			http.ServeContent(writer, request, "", time.Now(), bytes.NewReader(content))
		}),
	)
	return server
}
