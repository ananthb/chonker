package ranger

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/sourcegraph/conc/stream"
)

// Do performs http.Request r using http.Client c and returns a http.Response.
// The response body is fetched using HTTP Range requests.
// Each sub-request fetches a chunk of chunkSize bytes.
// Calls to Read() will return data piped sequentially, in-order, from fetched chunks.
// A maximum of workers chunks are fetched concurrently.
// If the request method is HEAD, the response is fetched in one go.
func Do(
	c *http.Client,
	r *http.Request,
	chunkSize, workers int64,
) (*http.Response, error) {
	ctx := r.Context()
	if c == nil {
		c = http.DefaultClient
	}
	if r == nil {
		return nil, errors.New("request cannot be nil")
	}
	if r.Method == http.MethodHead {
		return c.Do(r)
	}
	if chunkSize < 1 {
		return nil, errors.New("chunk size must be non-zero")
	}
	if workers < 1 {
		return nil, errors.New("buffer number must be non-zero")
	}

	req := r.Clone(ctx)
	req.Method = http.MethodHead
	req.Header.Del(headerNameRange)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	if ar := resp.Header.Get(headerNameAcceptRanges); ar != "bytes" {
		return nil, fmt.Errorf("%w, Accept-Ranges: %s", ErrRangeUnsupported, ar)
	}

	contentLength, err := strconv.ParseInt(resp.Header.Get(headerNameContentLength), 10, 64)
	if err != nil {
		return nil, fmt.Errorf(
			"unable to parse Content-Length header %s: %w",
			resp.Header.Get(headerNameContentLength),
			err,
		)
	}

	requestedRange := r.Header.Get(headerNameRange)
	chunks, err := ParseRange(requestedRange, contentLength)
	if err != nil {
		return nil, fmt.Errorf(
			"error parsing requested range %s: %w",
			requestedRange,
			err,
		)
	}

	headers := resp.Header.Clone()

	switch len(chunks) {
	case 0:
		chunks = Chunks(chunkSize, 0, contentLength)
	case 1:
		cr, ok := chunks[0].ContentRange(contentLength)
		if !ok {
			return nil, errors.New("unable to generate Content-Range header")
		}
		headers.Set(headerNameContentRange, cr)
		contentLength = chunks[0].Length
	default:
		return nil, fmt.Errorf("ranger does not support fetching multiple ranges")
	}

	read, write := io.Pipe()
	remoteFile := &remoteFileReader{
		client: c,
		url:    r.URL,
		chunks: chunks,
		data:   read,
	}
	fetchers := stream.New().WithMaxGoroutines(int(workers))
	go remoteFile.fillBuffer(ctx, fetchers, write)

	rangeResponse := http.Response{
		Status:        resp.Status,
		StatusCode:    resp.StatusCode,
		Proto:         resp.Proto,
		ProtoMajor:    resp.ProtoMajor,
		ProtoMinor:    resp.ProtoMinor,
		ContentLength: contentLength,
		Header:        headers,
		Body:          remoteFile,
		Request:       r,
		TLS:           resp.TLS,
	}
	return &rangeResponse, nil
}

// NewClient returns a new http.Client that uses a ranging http.RoundTripper.
// Chunks are chunkSize bytes long. A maximum of workers chunks are fetched concurrently.
func NewClient(chunkClient *http.Client, chunkSize, workers int64) *http.Client {
	return &http.Client{
		Transport: NewRoundTripper(chunkClient, chunkSize, workers),
	}
}

// NewRoundTripper returns a new http.RoundTripper that fetches requests in chunks.
// Chunks are chunkSize bytes long. A maximum of workers chunks are fetched concurrently.
// If chunkClient is nil, http.DefaultClient is used.
func NewRoundTripper(chunkClient *http.Client, chunkSize, workers int64) http.RoundTripper {
	return &rangingTripper{
		chunkClient: chunkClient,
		chunkSize:   chunkSize,
		workers:     workers,
	}
}

type rangingTripper struct {
	chunkClient *http.Client
	chunkSize   int64
	workers     int64
}

func (s *rangingTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return Do(s.chunkClient, request, s.chunkSize, s.workers)
}
