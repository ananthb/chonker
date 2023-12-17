package chonker

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

var ErrRangeUnsupported = errors.New("chonker: server does not support range requests")

type remoteFileReader struct {
	*io.PipeReader

	client  *http.Client
	request *Request
}

func (r *remoteFileReader) fetchChunks(
	ctx context.Context,
	chunks []Chunk,
	fetchers *stream.Stream,
	writer *io.PipeWriter,
) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		writer.Close()
	}()
	defer fetchers.Wait()

	for _, chunk := range chunks {
		req := r.request.Clone(ctx)
		rangeHeader, ok := chunk.Range()
		if !ok {
			writer.CloseWithError(
				fmt.Errorf("chonker: unable to generate Range header for %#v", chunk),
			)
			return
		}
		req.Header.Set(headerNameRange, rangeHeader)
		fetchers.Go(func() stream.Callback {
			resp, err := r.client.Do(req) //nolint:bodyclose
			// Chunk download goroutine.
			return func() {
				if err != nil {
					if !errors.Is(err, context.Canceled) {
						writer.CloseWithError(err)
					}
					cancel()
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusPartialContent {
					writer.CloseWithError(fmt.Errorf("%w fetching range %s, got status %s",
						ErrRangeUnsupported, rangeHeader, resp.Status))
					cancel()
					return
				}
				if _, err := io.Copy(writer, resp.Body); err != nil {
					if !(errors.Is(err, context.Canceled) || errors.Is(err, io.ErrClosedPipe)) {
						writer.CloseWithError(err)
						return
					}
					cancel()
					return
				}
			}
		})
	}
}
