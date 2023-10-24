package ranger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"

	"github.com/ananthb/ringbuffer"
	"github.com/sourcegraph/conc/stream"
)

const (
	headerNameAcceptRanges  = "Accept-Ranges"
	headerNameContentLength = "Content-Length"
	headerNameContentRange  = "Content-Range"
	headerNameRange         = "Range"
)

var ErrRangeUnsupported = errors.New("server does not support range requests")

type remoteFileReader struct {
	// Constants. Don't touch.
	client *http.Client
	url    *url.URL
	chunks []Chunk

	cancelFetch context.CancelFunc
	fetchDone   atomic.Bool
	fetchErr    atomic.Pointer[error]

	// Concurrent data buffer.
	buf *ringbuffer.RingBuffer
}

func (r *remoteFileReader) Read(p []byte) (int, error) {
	if err := r.fetchErr.Load(); err != nil {
		r.fetchErr.Store(nil)
		return 0, *err
	}
	if r.buf.IsEmpty() {
		if r.fetchDone.Load() {
			return 0, io.EOF
		}
		// Wait for the buffer to be filled
		return 0, nil
	}
	n, err := r.buf.Read(p)
	if err != nil {
		if errors.Is(err, ringbuffer.ErrEmpty) {
			return n, nil
		}
		return n, err
	}
	return n, nil
}

func (r *remoteFileReader) Close() error {
	r.cancelFetch()
	if err := r.fetchErr.Load(); err != nil {
		r.fetchErr.Store(nil)
		return *err
	}
	return nil
}

func (r *remoteFileReader) fillBuffer(ctx context.Context, fetchers *stream.Stream) {
	go func() {
		<-ctx.Done()
		r.fetchDone.Store(true)
	}()
	defer r.cancelFetch()
	defer fetchers.Wait()
	for _, rn := range r.chunks {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url.String(), nil)
		if err != nil && !errors.Is(err, context.Canceled) {
			r.fetchErr.Store(&err)
			return
		}
		rangeHeader, ok := rn.Range()
		if !ok {
			err := fmt.Errorf("unable to generate Range header for %#v", rn)
			r.fetchErr.Store(&err)
			return
		}
		req.Header.Set(headerNameRange, rangeHeader)
		fetchers.Go(func() stream.Callback {
			resp, err := r.client.Do(req) //nolint:bodyclose
			// Chunk download goroutine.
			return func() {
				if err != nil {
					if !errors.Is(err, context.Canceled) {
						r.fetchErr.Store(&err)
					}
					r.cancelFetch()
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusPartialContent {
					err := fmt.Errorf("%w fetching range %#v, got status %s",
						ErrRangeUnsupported, rn, resp.Status)
					r.fetchErr.Store(&err)
					r.cancelFetch()
					return
				}
				for {
					n, err := io.Copy(r.buf, resp.Body)
					if err == nil || errors.Is(err, context.Canceled) {
						return
					}
					if errors.Is(err, ringbuffer.ErrFull) {
						continue
					}
					if n != resp.ContentLength {
						err = io.ErrShortWrite
					}
					// err is non-nil and not ringbuffer.ErrFull or context.Canceled.
					r.fetchErr.Store(&err)
					r.cancelFetch()
					return
				}
			}
		})
	}
}
