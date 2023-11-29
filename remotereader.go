package ranger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

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
	*io.PipeReader

	// Constants. Don't touch.
	client *http.Client
	url    *url.URL
	chunks []Chunk
}

func (r *remoteFileReader) fetchChunks(
	ctx context.Context,
	fetchers *stream.Stream,
	w *io.PipeWriter,
) {
	ctx, cancelFetch := context.WithCancel(ctx)
	defer cancelFetch()
	go func() {
		<-ctx.Done()
		w.Close()
	}()
	defer fetchers.Wait()

	for _, rn := range r.chunks {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url.String(), nil)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				w.CloseWithError(err)
			}
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
					cancelFetch()
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusPartialContent {
					w.CloseWithError(fmt.Errorf("%w fetching range %#v, got status %s",
						ErrRangeUnsupported, rn, resp.Status))
					cancelFetch()
					return
				}
				if _, err := io.Copy(w, resp.Body); err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, io.ErrClosedPipe) {
						cancelFetch()
						return
					}
					w.CloseWithError(err)
					cancelFetch()
				}
			}
		})
	}
}
