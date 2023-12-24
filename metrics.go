package chonker

import (
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/metrics"
)

var (
	// StatsForNerds exposes Prometheus metrics for chonker requests.
	// Metric names are prefixed with "chonker_".
	// Metrics are labeled with and grouped by request host URL.
	StatsForNerds = metrics.NewSet()

	hostMetricsMap = sync.Map{}
)

type hostMetrics struct {
	requestsTotal               *metrics.Counter
	requestDurationSeconds      *metrics.Histogram
	requestSizeBytes            *metrics.Histogram
	requestChunkDurationSeconds *metrics.Histogram
	requestChunkSizeBytes       *metrics.Histogram
}

func newHostMetrics(host string) *hostMetrics {
	return &hostMetrics{
		requestsTotal: StatsForNerds.NewCounter(
			fmt.Sprintf("chonker_http_requests_total{host=\"%s\"}", host),
		),
		requestDurationSeconds: StatsForNerds.NewHistogram(
			fmt.Sprintf("chonker_http_request_duration_seconds{host=\"%s\"}", host),
		),
		requestSizeBytes: StatsForNerds.NewHistogram(
			fmt.Sprintf("chonker_http_request_size_bytes{host=\"%s\"}", host),
		),
		requestChunkDurationSeconds: StatsForNerds.NewHistogram(
			fmt.Sprintf("chonker_http_request_chunk_duration_seconds{host=\"%s\"}", host),
		),
		requestChunkSizeBytes: StatsForNerds.NewHistogram(
			fmt.Sprintf("chonker_http_request_chunk_size_bytes{host=\"%s\"}", host),
		),
	}
}

func getOrCreateHostMetrics(host string) *hostMetrics {
	v, ok := hostMetricsMap.Load(host)
	if ok {
		return v.(*hostMetrics)
	}
	m := newHostMetrics(host)
	hostMetricsMap.Store(host, m)
	return m
}
