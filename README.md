# Chonker

[![Chonker](chonker.jpg)](https://www.freepik.com/free-vector/hand-drawn-fat-cat-cartoon-illustration_58564188.htm)

[![Go Reference](https://pkg.go.dev/badge/github.com/ananthb/chonker.svg)](https://pkg.go.dev/github.com/ananthb/chonker)

*Download large files as parallel chunks using HTTP Range Requests in Go.*

[![Go](https://github.com/ananthb/chonker/actions/workflows/go.yml/badge.svg)](https://github.com/ananthb/chonker/actions/workflows/go.yml)

Chonker works on Go 1.19, oldstable, and stable releases.

## What does Chonker do?

Chonker speeds up downloads from cloud services like Amazon S3 & CloudFront.
It does this in two ways.

1. Download small pieces of a file (a.k.a a chunk) using
   [HTTP Range](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Range)
   requests.
2. Download chunks in parallel.

## Why?

Chonker allows CDN services to cache and serve files even if the entire
file is bigger than the individual object cache limit.

It also overcomes the per-connection limit that blob storage services often have
by opening connections in parallel.

The Go standard library
[HTTP Client](https://pkg.go.dev/net/http#hdr-Clients_and_Transports)
downloads files as buffered streams of bytes. The Client fetches bytes into a
request buffer as fast as it can, and you read bytes from the buffer as fast as
you can.

*Why is this a problem?*

This works great when one beefy connection to an origin server can use the entire
available network bandwidth between you and it. Blob file services like Amazon S3
and caching CDNs like Amazon CloudFront impose per-connection limits, but
support an almost unlimited number of connections from each client.

If you are downloading a large file from S3, its almost always better
to download the file in chunks, using parallel connections.

See the [S3 Developer Guide](https://docs.aws.amazon.com/whitepapers/latest/s3-optimizing-performance-best-practices/use-byte-range-fetches.html)
and the [CloudFront Developer Guide](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/RangeGETs.html)
for more information on cache sizes and parallel GETs.

## Use Chonker

Use `chonker.Do` to fetch a response for a request, or create a "chonky" `http.Transport`
that fetches requests using HTTP Range sub-requests.

Chonker integrates well with Go download libraries.
[Grab](https://github.com/cavaliergopher/grab) and other download managers
can use a `http.Client` with a "chonky" `http.Transport`.
In turn, Chonker functions accept HTTP clients that could provide automatic retries
or detailed logs. See
[Heimdall](https://github.com/gojek/heimdall) or [go-retryablehttp](https://github.com/hashicorp/go-retryablehttp)
for more.

### [Chonk](cmd/chonk)

Chonk is a Go program that uses the chonker library to download a URL into a local
file. Run `chonk -h` for usage details.

```shell
go build -o chonk ./cmd/chonk

./chonk https://example.com
```

### [`test.sh`](test.sh)

`test.sh` is a BASH shell script that exercises the `chonk` program with
a list of files of varying sizes. Run `test.sh -h` for usage details.

## License

The Chonker cat illustration is from [Freepik](https://www.freepik.com/free-vector/hand-drawn-fat-cat-cartoon-illustration_58564188.htm)

Chonker is a fork of [Ranger](https://github.com/sudhirj/ranger) and is available
under the terms of the MIT license.

See [LICENSE](LICENSE) for the full license text.
