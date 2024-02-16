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
// It is a wrapper around http.Request that adds support for ranged requests.
// If the server does not support range requests, the request fails.
// To succeed even if the server does not support range requests,
// use WithContinueSansRange.
type Request struct {
	*http.Request
	chunkSize uint64
	workers   uint

	continueWithoutRange bool
}

func (r Request) isValid() bool {
	return r.chunkSize > 0 && r.workers > 0
}

// WithContinueSansRange configures r to use ranged sub-requests opportunistically.
// If the server does not support range requests, the request succeeds anyway.
func (r *Request) WithContinueSansRange() *Request {
	r.continueWithoutRange = true
	return r
}

// NewRequestWithContext returns a new Request.
// It is a wrapper around http.NewRequestWithContext that adds support for ranged requests.
// A ranged request is a request that is fetched in chunks using several HTTP requests.
// Chunks are chunkSize bytes long. A maximum of workers chunks are fetched concurrently.
func NewRequestWithContext(
	ctx context.Context,
	method, url string,
	body io.Reader,
	chunkSize uint64, workers uint,
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
	chunkSize uint64,
	workers uint,
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
	probeReq.Header.Set(headerNameRange, Chunk{0, 1}.RangeHeader())
	probeResp, err := c.Do(probeReq)
	if err != nil {
		return nil, err
	}

	hostMetrics := getHostMetrics(r.URL.Host)

	if probeResp.StatusCode == http.StatusOK {
		if !r.continueWithoutRange {
			return nil, ErrRangeUnsupported
		}

		// The server does not support range requests but we're configured to continue anyway.
		// Return the response as-is.
		hostMetrics.requestsTotal.Inc()
		return probeResp, nil
	}

	if probeResp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("unexpected status code %d", probeResp.StatusCode)
	}

	crHeader := probeResp.Header.Get(headerNameContentRange)
	_, contentLength, err := ParseContentRange(crHeader)
	if err != nil {
		return nil, fmt.Errorf("chonker: error parsing Content-Range header %s: %w", crHeader, err)
	}

	reqRangeVal := r.Header.Get(headerNameRange)
	var chunks []Chunk

	if reqRangeVal == "" {
		chunks = Chunks(r.chunkSize, 0, contentLength)

		// Remove partial response status code and Content-Range header from the response.
		probeResp.Header.Del(headerNameContentRange)
		probeResp.StatusCode = http.StatusOK
		probeResp.Status = http.StatusText(http.StatusOK)
	} else {
		// The original request had a Range header.
		// Fetch the requested range.
		// Add the Content-Range header to the generated response.
		cs, err := ParseRange(reqRangeVal, contentLength)
		if err != nil {
			return nil, fmt.Errorf(
				"chonker: error parsing requested range %s: %w",
				reqRangeVal,
				err,
			)
		} else if len(cs) > 1 {
			return nil, ErrMultipleRangesUnsupported
		}
		requestedRange := cs[0]

		// Add partial response status and Content-Range header to the response.
		probeResp.Header.Set(headerNameContentRange, requestedRange.ContentRangeHeader(contentLength))
		probeResp.StatusCode = http.StatusPartialContent
		probeResp.Status = http.StatusText(http.StatusPartialContent)

		// Set content length to the length of the requested range.
		contentLength = requestedRange.Length
		chunks = Chunks(r.chunkSize, requestedRange.Start, requestedRange.Start+requestedRange.Length)
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
		ContentLength: int64(contentLength),
		Header:        probeResp.Header,
		Body:          remoteFile,
		Request:       r.Request,
	}

	hostMetrics.requestsTotal.Inc()
	return &rangeResponse, nil
}

// NewClient returns a new http.Client configured with a http.RoundTripper transport
// that fetches requests in chunks.
func NewClient(c *http.Client, chunkSize uint64, workers uint) (*http.Client, error) {
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
func NewRoundTripper(c *http.Client, chunkSize uint64, workers uint) (http.RoundTripper, error) {
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
