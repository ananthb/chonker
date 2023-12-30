package chonker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
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
	start := time.Now()
	downloadedBytes := atomic.Int64{}
	m := getHostMetrics(r.request.URL.Host)
	defer m.requestDurationSeconds.UpdateDuration(start)
	defer m.requestSizeBytes.Update(float64(downloadedBytes.Load()))
	defer m.requestsTotal.Inc()

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
			chunkStart := time.Now()
			resp, err := r.client.Do(req) //nolint:bodyclose
			return func() {
				if err != nil {
					cancel()
					if !errors.Is(err, context.Canceled) {
						writer.CloseWithError(err)
					}
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusPartialContent {
					cancel()
					writer.CloseWithError(fmt.Errorf("%w fetching range %s, got status %s",
						ErrRangeUnsupported, resp.Request.Header.Get(headerNameRange), resp.Status))
					return
				}
				n, err := io.Copy(writer, resp.Body)
				if err != nil {
					cancel()
					if !errors.Is(err, context.Canceled) && !errors.Is(err, io.ErrClosedPipe) {
						writer.CloseWithError(err)
						return
					}
				}
				// Update chunk metrics
				m := getHostMetrics(r.request.URL.Host)
				m.requestChunkDurationSeconds.UpdateDuration(chunkStart)
				m.requestChunkSizeBytes.Update(float64(n))
				downloadedBytes.Add(n)
			}
		})
	}
}
