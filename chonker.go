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

// Do performs Request r using http.Client c and returns a http.Response.
// If c is nil, http.DefaultClient is used.
// The returned Response.Body is a ReadCloser that reads from the remote file in chunks.
// Chunks are fetched parallelly and are written to the ReadCloser in order.
// If r.Method is HEAD, the response is fetched without ranging, in one request.
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

// NewClient returns a new http.Client that uses a ranging http.RoundTripper.
// Chunks are chunkSize bytes long. A maximum of workers chunks are fetched concurrently.
func NewClient(chunkClient *http.Client, chunkSize int64, workers int) (*http.Client, error) {
	transport, err := NewRoundTripper(chunkClient, chunkSize, workers)
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
// Chunks are chunkSize bytes long. A maximum of workers chunks are fetched concurrently.
// If chunkClient is nil, http.DefaultClient is used.
func NewRoundTripper(
	chunkClient *http.Client,
	chunkSize int64, workers int,
) (http.RoundTripper, error) {
	if chunkSize < 1 || workers < 1 {
		return nil, ErrInvalidArgument
	}
	return roundTripper(func(r *http.Request) (*http.Response, error) {
		req := Request{
			Request:   r,
			chunkSize: chunkSize,
			workers:   workers,
		}
		return Do(chunkClient, &req)
	}), nil
}
