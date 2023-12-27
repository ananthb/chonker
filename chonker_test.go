package chonker

import (
	"bytes"
	"context"
	"fmt"
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
		workers  int
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
			name:        "start at 42",
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
			name: "8 byte chunk",
			request: Request{
				chunkSize: 8,
				workers:   128,
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
			name:    "1MiB chunks",
			content: makeData(16 * 1024 * 1024), // 16MiB
			request: Request{
				chunkSize: 1024 * 1024,
				workers:   8,
			},
			expected: expected{body: content, contentLength: int64(len(content))},
		},
		{
			name: "single worker",
			request: Request{
				chunkSize: 1024,
				workers:   1,
			},
			expected: expected{body: content, contentLength: int64(len(content))},
		},
		{
			name: "test timeout",
			request: Request{
				chunkSize: 1,
				workers:   1,
			},
			content: makeData(1024 * 1024), // 1MiB
			timeout: 100 * time.Millisecond,
		},
		{
			name: "invalid request",
			err:  true,
		},
		{
			name:        "invalid range",
			request:     Request{chunkSize: 1024, workers: 100},
			rangeHeader: "bytes=100-50",
			err:         true,
		},
		{
			name:        "error fetching multiple ranges",
			request:     Request{chunkSize: 1024, workers: 100},
			rangeHeader: "bytes=100-200,300-400",
			err:         true,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if testCase.content == nil {
				testCase.content = content
			}
			server := makeHttptestServer(content)
			defer server.Close()

			ctx := context.Background()
			var cancel context.CancelFunc
			if testCase.timeout > 0 {
				ctx, cancel = context.WithTimeout(ctx, testCase.timeout)
			} else {
				ctx, cancel = context.WithCancel(ctx)
			}
			defer cancel()

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
			if testCase.timeout > 0 {
				<-ctx.Done()
				assert.NoError(t, err)
			} else {
				if testCase.err {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					defer resp.Body.Close()
					assert.Equal(t, testCase.expected.contentLength, resp.ContentLength)
					assert.Equal(t, testCase.expected.contentRangeHeader, resp.Header.Get("Content-Range"))
					assert.NoError(t, iotest.TestReader(resp.Body, testCase.expected.body))
				}
			}
		})
	}
}

func TestDo_InvalidRequest(t *testing.T) {
	_, err := NewRequest(http.MethodGet, "http://user:abc{DEf1=ghi@example.com", nil, 64, 8)
	assert.Error(t, err)
}

func TestDo_NilRequest(t *testing.T) {
	resp, err := Do(nil, nil)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestDo_PreserveRequestAttributes(t *testing.T) {
	content := makeData(512)
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Header.Get("Range") != "bytes=0-1" {
				assert.Equal(t, http.MethodPut, r.Method)
			}
			assert.Equal(t, "test", r.Header.Get("X-Test"))
			http.ServeContent(w, r, "", time.Now(), bytes.NewReader(content))
		}),
	)
	defer server.Close()

	req, err := NewRequest(http.MethodPut, server.URL, nil, 64, 8)
	assert.NoError(t, err)
	req.Header.Set("X-Test", "test")

	resp, err := Do(nil, req)
	assert.NoError(t, err)
	defer resp.Body.Close()
}

func TestDo_HeadRequest(t *testing.T) {
	content := makeData(1024 * 10)
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodHead, r.Method)
			http.ServeContent(w, r, "", time.Now(), bytes.NewReader(content))
		}),
	)
	defer server.Close()

	req, err := NewRequest(http.MethodHead, server.URL, nil, 64, 8)
	assert.NoError(t, err)

	resp, err := Do(nil, req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, int64(len(content)), resp.ContentLength)
}

func TestDo_InvalidContentLength(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", "not a number")
		}),
	)
	defer server.Close()

	req, err := NewRequest(http.MethodGet, server.URL, nil, 64, 8)
	assert.NoError(t, err)

	_, err = Do(nil, req)
	assert.Error(t, err)
}

func TestDo_ChunkRequestNotSupported(t *testing.T) {
	content := makeData(1024)
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Probe request will succeed but chunk requests will fail.
			if rng := r.Header.Get("Range"); rng != "" && rng != "bytes=0-0" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			http.ServeContent(w, r, "", time.Now(), bytes.NewReader(content))
		}),
	)
	defer server.Close()

	req, err := NewRequest(http.MethodGet, server.URL, nil, 64, 8)
	assert.NoError(t, err)

	resp, err := Do(nil, req)
	assert.NoError(t, err)

	defer resp.Body.Close()
	assert.Error(t, iotest.TestReader(resp.Body, content))
}

func TestNewClient(t *testing.T) {
	content := makeData(1024 * 10)
	server := makeHttptestServer(content)
	defer server.Close()

	_, err := NewClient(nil, 1024, 100)
	assert.NoError(t, err)

	_, err = NewClient(nil, 0, 100)
	assert.Error(t, err)

	_, err = NewClient(nil, 1024, 0)
	assert.Error(t, err)
}

func TestNewRoundTripper(t *testing.T) {
	content := makeData(1024 * 10)
	server := makeHttptestServer(content)
	defer server.Close()

	transport, err := NewRoundTripper(nil, 1024, 100)
	assert.NoError(t, err)

	client := &http.Client{Transport: transport}
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	assert.NoError(t, err)

	resp, err := client.Do(req)
	assert.NoError(t, err)

	defer resp.Body.Close()

	assert.NoError(t, iotest.TestReader(resp.Body, content))
}

func BenchmarkDo(b *testing.B) {
	content := makeData(1024) // 1KiB
	server := makeHttptestServer(content)
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	assert.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := Do(nil, &Request{
			Request:   req,
			chunkSize: 64,
			workers:   8,
		})
		assert.NoError(b, err)
		defer resp.Body.Close()
		assert.NoError(b, iotest.TestReader(resp.Body, content))
	}
}

func ExampleNewRequest() {
	req, err := NewRequest(http.MethodGet, "http://example.com", nil, 64, 8)
	if err != nil {
		panic(err)
	}
	fmt.Println(req.URL)
	// Output: http://example.com
}

func ExampleDo() {
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		panic(err)
	}

	resp, err := Do(nil, &Request{
		Request:   req,
		chunkSize: 64,
		workers:   8,
	})
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
}

func ExampleNewRoundTripper() {
	transport, err := NewRoundTripper(nil, 64, 8)
	if err != nil {
		panic(err)
	}

	// Use the transport with a http.Client.
	client := &http.Client{Transport: transport}
	resp, err := client.Get("http://example.com")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
}

func ExampleNewClient() {
	client, err := NewClient(nil, 64, 8)
	if err != nil {
		panic(err)
	}

	// Use the client.
	resp, err := client.Get("http://example.com")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
}

func makeData(size int) []byte {
	rnd := rand.New(rand.NewSource(42))
	content := make([]byte, size)
	rnd.Read(content)
	return content
}

func makeHttptestServer(content []byte) *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeContent(w, r, "", time.Now(), bytes.NewReader(content))
		}),
	)
}
