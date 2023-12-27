// Package chonker implements automatic ranged HTTP requests.
//
// A ranged request is a request that is fetched in chunks using several HTTP requests.
// Chunks are fetched in separate goroutines by sending HTTP Range
// requests to the server.
// Chunks are then concatenated and returned as a single io.Reader.
// Chunks are chunkSize bytes long.
// A maximum of workers chunks are fetched concurrently.
// If the server does not support range requests, the request fails.
package chonker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/sourcegraph/conc/stream"
)

var (
	ErrInvalidArgument = errors.New(
		"chonker: chunkSize and workers must be greater than zero",
	)
	ErrMultipleRangesUnsupported = errors.New("chonker: multiple ranges not supported")
)

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
		return nil, errors.New("chonker: request cannot be nil")
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

	// Probe the server to see if it supports range requests.
	// If it does, fetch the requested range.
	// We'd normally do a HEAD request here, but some servers don't support HEAD requests.
	// So we do a GET request for the first byte of the file.
	probeReq := r.Clone(ctx)
	probeReq.Method = http.MethodGet
	firstByteRange, _ := Chunk{0, 1}.RangeHeader()
	probeReq.Header.Set(headerNameRange, firstByteRange)
	probeResp, err := c.Do(probeReq)
	if err != nil {
		return nil, err
	}
	if probeResp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("%w, status code %d", ErrRangeUnsupported, probeResp.StatusCode)
	}

	crHeader := probeResp.Header.Get(headerNameContentRange)
	_, contentLength, err := ParseContentRange(crHeader)
	if err != nil {
		return nil, fmt.Errorf("chonker: error parsing Content-Range header %s: %w", crHeader, err)
	}
	// Remove the Content-Range header from the response.
	// We'll add it back later if we are fetching a range.
	probeResp.Header.Del(headerNameContentRange)

	requestedRange := r.Header.Get(headerNameRange)
	var chunks []Chunk
	headers := probeResp.Header.Clone()

	if requestedRange == "" {
		chunks = Chunks(r.chunkSize, 0, contentLength)
	} else {
		// The original request had a Range header.
		// Fetch the requested range.
		// Add the Content-Range header to the generated response.
		var err error
		if chunks, err = ParseRange(requestedRange, contentLength); err != nil {
			return nil, fmt.Errorf(
				"chonker: error parsing requested range %s: %w",
				requestedRange,
				err,
			)
		} else if len(chunks) > 1 {
			return nil, ErrMultipleRangesUnsupported
		}
		cr, ok := chunks[0].ContentRangeHeader(contentLength)
		if !ok {
			return nil, errors.New("chonker: unable to generate Content-Range header")
		}
		headers.Set(headerNameContentRange, cr)
		contentLength = chunks[0].Length
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
