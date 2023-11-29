package ranger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

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
	client  *http.Client
	request *Request
	chunks  []Chunk
}

func (r *remoteFileReader) fetchChunks(
	ctx context.Context,
	fetchers *stream.Stream,
	w *io.PipeWriter,
) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		w.Close()
	}()
	defer fetchers.Wait()

	for _, chunk := range r.chunks {
		req := r.request.Clone(ctx)
		rangeHeader, ok := chunk.Range()
		if !ok {
			w.CloseWithError(fmt.Errorf("unable to generate Range header for %#v", chunk))
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
					cancel()
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusPartialContent {
					w.CloseWithError(fmt.Errorf("%w fetching range %#v, got status %s",
						ErrRangeUnsupported, chunk, resp.Status))
					cancel()
					return
				}
				if _, err := io.Copy(w, resp.Body); err != nil {
					if !(errors.Is(err, context.Canceled) || errors.Is(err, io.ErrClosedPipe)) {
						w.CloseWithError(err)
						return
					}
					cancel()
					return
				}
			}
		})
	}
}
