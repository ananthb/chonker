package chonker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/sourcegraph/conc/stream"
)

var ErrInvalidArgument = errors.New("chonker: chunkSize and workers must be greater than zero")

// Request is a ranged http.Request.
type Request struct {
	*http.Request
	chunkSize int64
	workers   int
}

func (r Request) isValid() bool {
	return r.chunkSize > 0 && r.workers > 0
}

// NewRequestWithContext returns a new Request.
// It is a wrapper around http.NewRequestWithContext that adds support for ranged requests.
// A ranged request is a request that is fetched in chunks using several HTTP requests.
// Chunks are chunkSize bytes long. A maximum of workers chunks are fetched concurrently.
func NewRequestWithContext(
	ctx context.Context,
	method, url string,
	body io.Reader,
	chunkSize int64, workers int,
) (*Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	r := &Request{
		Request:   req,
		chunkSize: chunkSize,
		workers:   workers,
	}
	if !r.isValid() {
		return nil, ErrInvalidArgument
	}
	return r, nil
}

// NewRequest returns a new Request.
// See NewRequestWithContext for more information.
func NewRequest(
	method, url string,
	body io.Reader,
	chunkSize int64,
	workers int,
) (*Request, error) {
	return NewRequestWithContext(context.Background(), method, url, body, chunkSize, workers)
}

// Do sends an HTTP request and returns an HTTP response, following policy (such as redirects,
// cookies, auth) as configured on the client.
// It is a wrapper around http.Client.Do that adds support for ranged requests.
// A ranged request is a request that is fetched in chunks using several HTTP requests.
// Chunks are chunkSize bytes long. A maximum of workers chunks are fetched concurrently.
// HTTP HEAD requests are not fetched in chunks.
func Do(c *http.Client, r *Request) (*http.Response, error) {
	if r == nil || r.Request == nil {
		return nil, errors.New("request cannot be nil")
	}
	if !r.isValid() {
		return nil, ErrInvalidArgument
	}
	if c == nil {
		c = http.DefaultClient
	}
	if r.Method == http.MethodHead {
		return c.Do(r.Request)
	}

	ctx := r.Context()

	probeReq := r.Clone(ctx)
	probeReq.Method = http.MethodHead
	probeReq.Header.Del(headerNameRange)
	probeResp, err := c.Do(probeReq)
	if err != nil {
		return nil, err
	}

	if ar := probeResp.Header.Get(headerNameAcceptRanges); ar != "bytes" {
		return nil, fmt.Errorf("%w, Accept-Ranges: %s", ErrRangeUnsupported, ar)
	}

	contentLength, err := strconv.ParseInt(probeResp.Header.Get(headerNameContentLength), 10, 64)
	if err != nil {
		return nil, fmt.Errorf(
			"unable to parse Content-Length header %s: %w",
			probeResp.Header.Get(headerNameContentLength),
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

	headers := probeResp.Header.Clone()

	switch len(chunks) {
	case 0:
		chunks = Chunks(r.chunkSize, 0, contentLength)
	case 1:
		cr, ok := chunks[0].ContentRange(contentLength)
		if !ok {
			return nil, errors.New("unable to generate Content-Range header")
		}
		headers.Set(headerNameContentRange, cr)
		contentLength = chunks[0].Length
	default:
		return nil, errors.New("ranger: multiple ranges not supported")
	}

	read, write := io.Pipe()
	remoteFile := &remoteFileReader{
		PipeReader: read,
		client:     c,
		request:    r,
	}
	fetchers := stream.New().WithMaxGoroutines(int(r.workers))
	go remoteFile.fetchChunks(ctx, chunks, fetchers, write)

	rangeResponse := http.Response{
		Status:     probeResp.Status,
		StatusCode: probeResp.StatusCode,
		Proto:      probeResp.Proto,
		ProtoMajor: probeResp.ProtoMajor,
		ProtoMinor: probeResp.ProtoMinor,
		TLS:        probeResp.TLS,

		// Synthesised fields.
		ContentLength: contentLength,
		Header:        headers,
		Body:          remoteFile,
		Request:       r.Request,
	}
	return &rangeResponse, nil
}

// NewClient returns a new http.Client configured with a http.RoundTripper transport
// that fetches requests in chunks.
func NewClient(c *http.Client, chunkSize int64, workers int) (*http.Client, error) {
	transport, err := NewRoundTripper(c, chunkSize, workers)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: transport,
	}, nil
}

type roundTripper func(*http.Request) (*http.Response, error)

func (r roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return r(req)
}

// NewRoundTripper returns a new http.RoundTripper that fetches requests in chunks.
func NewRoundTripper(c *http.Client, chunkSize int64, workers int) (http.RoundTripper, error) {
	if chunkSize < 1 || workers < 1 {
		return nil, ErrInvalidArgument
	}
	return roundTripper(func(r *http.Request) (*http.Response, error) {
		req := Request{
			Request:   r,
			chunkSize: chunkSize,
			workers:   workers,
		}
		return Do(c, &req)
	}), nil
}
