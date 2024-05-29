package chonker

import (
	"fmt"

	"github.com/VictoriaMetrics/metrics"
)

// StatsForNerds exposes Prometheus metrics for chonker requests.
// Metric names are prefixed with "chonker_".
// Metrics are labeled with and grouped by request host URL.
//
// Rhe following metrics are exposed for a request to https://example.com:
//
// chonker_http_requests_fetching{host="example.com"}
// chonker_http_requests_total{host="example.com"}
// chonker_http_requests_total{host="example.com",range="false"}
// chonker_http_request_chunks_fetching{host="example.com",stage="do"}
// chonker_http_request_chunks_fetching{host="example.com",stage="copy"}
// chonker_http_request_chunks_total{host="example.com"}
// chonker_http_request_chunk_duration_seconds{host="example.com"}
// chonker_http_request_chunk_bytes{host="example.com"}
//
// You can surface these metrics in your application using the
// [metrics.RegisterSet] function.
//
// [metrics.RegisterSet]: https://pkg.go.dev/github.com/VictoriaMetrics/metrics#RegisterSet
var StatsForNerds = metrics.NewSet()

type hostMetrics struct {
	// requestsFetching is the number of currently active requests to a host.
	requestsFetching *metrics.Gauge
	// requestsTotal is the total number of requests completed to a host.
	requestsTotal *metrics.Counter
	// requestsTotalSansRange is the total number of requests completed to a host
	// that did not use range requests.
	requestsTotalSansRange *metrics.Counter
	// requestChunksFetching is the number of currently active request chunks to a host.
	requestChunksFetchingStageDo   *metrics.Gauge
	requestChunksFetchingStageCopy *metrics.Gauge
	// requestChunksTotal is the total number of request chunks completed to a host.
	requestChunksTotal *metrics.Counter
	// requestChunkDurationSeconds measures the duration of request chunks to a host.
	requestChunkDurationSeconds *metrics.Histogram
	// requestChunkBytes measures the number of bytes fetched in request chunks to a host.
	requestChunkBytes *metrics.Histogram
}

func getHostMetrics(host string) *hostMetrics {
	return &hostMetrics{
		requestsFetching: StatsForNerds.GetOrCreateGauge(
			fmt.Sprintf(`chonker_http_requests_fetching{host="%s"}`, host), nil,
		),
		requestsTotal: StatsForNerds.GetOrCreateCounter(
			fmt.Sprintf(`chonker_http_requests_total{host="%s"}`, host),
		),
		requestsTotalSansRange: StatsForNerds.GetOrCreateCounter(
			fmt.Sprintf(`chonker_http_requests_total{host="%s",range="false"}`, host),
		),
		requestChunksFetchingStageDo: StatsForNerds.GetOrCreateGauge(
			fmt.Sprintf(`chonker_http_request_chunks_fetching{host="%s",stage="do"}`, host), nil,
		),
		requestChunksFetchingStageCopy: StatsForNerds.GetOrCreateGauge(
			fmt.Sprintf(`chonker_http_request_chunks_fetching{host="%s",stage="copy"}`, host), nil,
		),
		requestChunksTotal: StatsForNerds.GetOrCreateCounter(
			fmt.Sprintf(`chonker_http_request_chunks_total{host="%s"}`, host),
		),
		requestChunkDurationSeconds: StatsForNerds.GetOrCreateHistogram(
			fmt.Sprintf(`chonker_http_request_chunk_duration_seconds{host="%s"}`, host),
		),
		requestChunkBytes: StatsForNerds.GetOrCreateHistogram(
			fmt.Sprintf(`chonker_http_request_chunk_bytes{host="%s"}`, host),
		),
	}
}
