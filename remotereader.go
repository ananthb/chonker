package ranger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"

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

	data *io.PipeReader
}

func (r *remoteFileReader) Read(p []byte) (int, error) {
	if r.fetchDone.Load() {
		return 0, io.EOF
	}
	return r.data.Read(p)
}

func (r *remoteFileReader) Close() error {
	r.cancelFetch()
	return nil
}

func (r *remoteFileReader) fillBuffer(
	ctx context.Context,
	fetchers *stream.Stream,
	w *io.PipeWriter,
) {
	defer r.fetchDone.Store(true)
	defer w.Close()
	defer r.cancelFetch()
	defer fetchers.Wait()

	for _, rn := range r.chunks {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url.String(), nil)
		if err != nil && !errors.Is(err, context.Canceled) {
			w.CloseWithError(err)
			return
		}
		rangeHeader, ok := rn.Range()
		if !ok {
			w.CloseWithError(fmt.Errorf("unable to generate Range header for %#v", rn))
			return
		}
		req.Header.Set(headerNameRange, rangeHeader)
		fetchers.Go(func() stream.Callback {
			resp, err := r.client.Do(req) //nolint:bodyclose
			// Chunk download goroutine.
			return func() {
				if err != nil {
					if !errors.Is(err, context.Canceled) {
						w.CloseWithError(err)
					}
					r.cancelFetch()
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusPartialContent {
					w.CloseWithError(fmt.Errorf("%w fetching range %#v, got status %s",
						ErrRangeUnsupported, rn, resp.Status))
					r.cancelFetch()
					return
				}
				if _, err := io.Copy(w, resp.Body); err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					w.CloseWithError(err)
					r.cancelFetch()
					return
				}
			}
		})
	}
}
