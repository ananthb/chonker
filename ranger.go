package ranger

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ananthb/ringbuffer"
	"github.com/sourcegraph/conc/stream"
)

// Do performs http.Request r using http.Client c and returns a http.Response.
// The response body is fetched using HTTP Range requests.
// Each sub-request fetches a chunk of chunkSize bytes.
// Fetched chunks are written to a ring buffer of bufferNum chunks.
// The buffer is filled asynchronously as the reader reads the response body.
// If the request method is HEAD, the response is fetched in one go.
func Do(
	c *http.Client,
	r *http.Request,
	chunkSize, bufferNum int64,
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
		return nil, errors.New("chunk size must be greater than 0")
	}
	if bufferNum < 1 {
		return nil, errors.New("buffer number must be greater than 0")
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
	parsedRange, err := ParseRange(requestedRange, contentLength)
	if err != nil || len(parsedRange) > 1 {
		err = fmt.Errorf(
			"unable to parse requested Range header %s correctly: %w",
			requestedRange,
			err,
		)
		return nil, err
	}

	fetchRange := Chunk{Length: contentLength}
	if len(parsedRange) > 0 {
		fetchRange = parsedRange[0]
	}

	fetchCtx, cancelFetch := context.WithCancel(ctx)
	chunks := Chunks(chunkSize, fetchRange.Start, fetchRange.Start+fetchRange.Length)
	remoteFile := &remoteFileReader{
		client:      c,
		url:         r.URL,
		chunks:      chunks,
		cancelFetch: cancelFetch,
		buf:         ringbuffer.New(int(bufferNum * chunkSize)),
	}
	fetchers := stream.New().WithMaxGoroutines(int(bufferNum))
	go remoteFile.fillBuffer(fetchCtx, fetchers)

	headers := resp.Header.Clone()
	cr, ok := fetchRange.ContentRange(contentLength)
	if !ok {
		return nil, errors.New("unable to generate Content-Range header")
	}
	headers.Set(headerNameContentRange, cr)

	rangeResponse := http.Response{
		Status:        resp.Status,
		StatusCode:    resp.StatusCode,
		Proto:         resp.Proto,
		ProtoMajor:    resp.ProtoMajor,
		ProtoMinor:    resp.ProtoMinor,
		ContentLength: fetchRange.Length,
		Header:        headers,
		Body:          remoteFile,
		Request:       r,
		TLS:           resp.TLS,
	}
	return &rangeResponse, nil
}

// NewClient returns a new http.Client that uses a ranging http.RoundTripper.
// chunkSize specifies the size of a chunk in bytes.
// bufferNum specifies the number of chunks to buffer in memory.
func NewClient(chunkClient *http.Client, chunkSize, bufferNum int64) *http.Client {
	return &http.Client{
		Transport: NewRoundTripper(chunkClient, chunkSize, bufferNum),
	}
}

// NewRoundTripper returns a new http.RoundTripper that fetches requests in chunks.
// chunkSize specifies the size of a chunk in bytes.
// bufferNum specifies the number of chunks to buffer in memory.
// If chunkClient is nil, http.DefaultClient is used.
// If buffer size in bytes is greater than the size of the file, the file is fetched in one request.
func NewRoundTripper(chunkClient *http.Client, chunkSize, bufferNum int64) http.RoundTripper {
	if chunkClient == nil {
		chunkClient = http.DefaultClient
	}
	return &rangingTripper{
		chunkClient: chunkClient,
		chunkSize:   chunkSize,
		bufferNum:   bufferNum,
	}
}

type rangingTripper struct {
	chunkClient *http.Client
	chunkSize   int64
	bufferNum   int64
}

func (s *rangingTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return Do(s.chunkClient, request, s.chunkSize, s.bufferNum)
}
