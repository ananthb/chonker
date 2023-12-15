# RANGER

[![Go Reference](https://pkg.go.dev/badge/github.com/sudhirj/ranger.svg)](https://pkg.go.dev/github.com/sudhirj/ranger)

*Download large files as parallel chunks using HTTP Range requests in Go.*

[![Go](https://github.com/sudhirj/ranger/actions/workflows/go.yml/badge.svg)](https://github.com/sudhirj/ranger/actions/workflows/go.yml)

Ranger works on Go 1.19, oldstable, and stable releases.

## Why?

The Go standard library
[HTTP Client](https://pkg.go.dev/net/http#hdr-Clients_and_Transports)
downloads files as buffered streams of bytes. The Client fetches bytes into its
internal buffer as fast as it can, and you read bytes off the buffer as fast as
you can.

### Why this is a problem

This works great when one fast connection to the origin server can effectively
use the network bandwidth between you and it. Blob file services like Amazon S3
and caching CDNs like Amazon CloudFront might have per-connection limits, but
support an almost unlimited number of connections between you and them.
If you are downloading a large file from S3, its almost always better
to download the file in chunks, using parallel connections.

## What does Ranger do?

Ranger speeds up transfers to Amazon S3 & CloudFront using two methods.

1. Download files in small chunks using [HTTP Range](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Range)
   requests.
2. Download chunks in parallel.

This allows CDN services to cache and serve cached chunks, even if the entire
file is bigger than the individual object cache limit.

See the [S3 Developer Guide](https://docs.aws.amazon.com/whitepapers/latest/s3-optimizing-performance-best-practices/use-byte-range-fetches.html)
and the [CloudFront Developer Guide](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/RangeGETs.html)
for more information on cache sizes and parallel GETs.

## Use Ranger

Use Ranger as an `io.ReadCloser`, an `http.Transport` that performs ranged
requests, or an `http.Client` that uses the afore-mentioned transport.

This means that Ranger integrates well on both sides.
[Grab](https://github.com/cavaliergopher/grab) and other download managers
can use a ranging client via the standard `http.Client` interface.
While the ranging `http.Client` can wrap around other HTTP Clients that could
provide automatic retries or better logging. See
[Heimdall](https://github.com/gojek/heimdall) or [go-retryablehttp](https://github.com/hashicorp/go-retryablehttp).
