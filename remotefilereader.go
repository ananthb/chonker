package chonker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

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
	// Update metrics
	m := getHostMetrics(r.request.URL.Host)
	m.requestsFetching.Inc()
	defer m.requestsFetching.Dec()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		writer.Close()
	}()
	defer fetchers.Wait()

	for _, chunk := range chunks {
		req := r.request.Clone(ctx)
		req.Header.Set(headerNameRange, chunk.RangeHeader())
		fetchers.Go(func() stream.Callback {
			m.requestChunksFetchingStageDo.Inc()
			defer m.requestChunksFetchingStageDo.Dec()
			defer m.requestChunksTotal.Inc()

			fetchStart := time.Now()
			resp, err := r.client.Do(req) //nolint:bodyclose

			return func() {
				m.requestChunksFetchingStageCopy.Inc()
				defer m.requestChunksFetchingStageCopy.Dec()

				if n, ok, err := copyChunk(writer, resp, err); !ok {
					cancel()
					if err != nil {
						writer.CloseWithError(err)
					}
				} else {
					m.requestChunkDurationSeconds.UpdateDuration(fetchStart)
					m.requestChunkBytes.Update(float64(n))
				}
			}
		})
	}
}

// copyChunk copies a chunk from the response body to the pipe writer.
// The first return value is the number of bytes copied.
// If the second return value is true, copying should continue.
// If false, copying should stop.
// The third return value is the error, if any.
func copyChunk(w io.Writer, resp *http.Response, err error) (int64, bool, error) {
	if err != nil {
		if errors.Is(err, context.Canceled) {
			err = nil
		}
		return 0, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return 0, false, fmt.Errorf("%w fetching range %s, got status %s",
			ErrRangeUnsupported, resp.Request.Header.Get(headerNameRange), resp.Status)
	}
	n, err := io.Copy(w, resp.Body)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, io.ErrClosedPipe) {
			err = nil
		}
		return 0, false, err
	}
	return n, true, nil
}
