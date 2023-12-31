package chonker

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"
)

var (
	// StatsForNerds exposes Prometheus metrics for chonker requests.
	// Metric names are prefixed with "chonker_".
	// Metrics are labeled with and grouped by request host URL.
	//
	// For example, the following metrics are exposed for a request to
	// https://example.com:
	//
	// chonker_http_requests_active{host="example.com"}
	// chonker_http_requests_total{host="example.com"}
	// chonker_http_request_duration_seconds{host="example.com"}
	// chonker_http_request_size_bytes{host="example.com"}
	// chonker_http_request_chunks_active{host="example.com"}
	// chonker_http_request_chunks_total{host="example.com"}
	// chonker_http_request_chunk_duration_seconds{host="example.com"}
	// chonker_http_request_chunk_size_bytes{host="example.com"}
	//
	// You can surface these metrics in your application using the
	// [metrics.RegisterSet] function.
	//
	// [metrics.RegisterSet]: https://pkg.go.dev/github.com/VictoriaMetrics/metrics#RegisterSet
	StatsForNerds = metrics.NewSet()

	hostMetricsMap = sync.Map{}
)

type hostMetrics struct {
	requestsActive         atomic.Int64
	requestsTotal          *metrics.Counter
	requestDurationSeconds *metrics.Histogram
	requestSizeBytes       *metrics.Histogram

	requestChunksActive         atomic.Int64
	requestChunksTotal          *metrics.Counter
	requestChunkDurationSeconds *metrics.Histogram
	requestChunkSizeBytes       *metrics.Histogram
}

func getHostMetrics(host string) *hostMetrics {
	m, ok := hostMetricsMap.Load(host)
	if ok {
		return m.(*hostMetrics)
	}

	hm := &hostMetrics{
		requestsTotal: StatsForNerds.NewCounter(
			fmt.Sprintf(`chonker_http_requests_total{host="%s"}`, host),
		),
		requestDurationSeconds: StatsForNerds.NewHistogram(
			fmt.Sprintf(`chonker_http_request_duration_seconds{host="%s"}`, host),
		),
		requestSizeBytes: StatsForNerds.NewHistogram(
			fmt.Sprintf(`chonker_http_request_size_bytes{host="%s"}`, host),
		),
		requestChunksTotal: StatsForNerds.NewCounter(
			fmt.Sprintf(`chonker_http_request_chunks_total{host="%s"}`, host),
		),
		requestChunkDurationSeconds: StatsForNerds.NewHistogram(
			fmt.Sprintf(`chonker_http_request_chunk_duration_seconds{host="%s"}`, host),
		),
		requestChunkSizeBytes: StatsForNerds.NewHistogram(
			fmt.Sprintf(`chonker_http_request_chunk_size_bytes{host="%s"}`, host),
		),
	}

	_ = StatsForNerds.NewGauge(
		fmt.Sprintf(`chonker_http_requests_active{host="%s"}`, host),
		func() float64 {
			return float64(hm.requestsActive.Load())
		},
	)
	_ = StatsForNerds.NewGauge(
		fmt.Sprintf(`chonker_http_request_chunks_active{host="%s"}`, host),
		func() float64 {
			return float64(hm.requestChunksActive.Load())
		},
	)

	hostMetricsMap.Store(host, hm)
	return hm
}
