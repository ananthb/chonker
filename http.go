package ranger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gotd/contrib/http_range"
)

// HTTPClient provides an interface allowing us to perform HTTP requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// RangingHTTPClient wraps another HTTP client to issue all requests in pre-defined chunks.
type RangingHTTPClient struct {
	client HTTPClient
	ranger Ranger
	HTTPClient
	parallelism int
}

func (rhc RangingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	contentLength, err := GetContentLength(req.URL, rhc.client)
	if err != nil {
		return nil, fmt.Errorf("error getting content length: %w", err)
	}

	loader := HTTPLoader(req.URL, rhc.client)
	remoteFile := NewRangedSource(contentLength, loader, rhc.ranger)
	reader := remoteFile.ParallelReader(context.Background(), rhc.parallelism)

	rangerHeader := req.Header.Get("Range")
	if rangerHeader != "" {
		ranges, err := http_range.ParseRange(rangerHeader, contentLength)
		if err != nil {
			return nil, err
		}
		if len(ranges) != 1 {
			return nil, errors.New("only single range supported")
		}
		reader = io.LimitReader(remoteFile.ParallelOffsetReader(context.Background(), rhc.parallelism, ranges[0].Start), ranges[0].Length)
	}

	combinedResponse := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Body:          io.NopCloser(reader),
		ContentLength: contentLength,
		Request:       req,
	}

	return combinedResponse, nil
}

// GetContentLengthViaHEAD returns the content length of the given URL, using the given HTTPClient. It
// uses a HEAD request to get the content length.
func GetContentLengthViaHEAD(url *url.URL, client HTTPClient) (int64, error) {
	headReq, err := http.NewRequest("HEAD", url.String(), nil)
	if err != nil {
		return 0, err
	}
	headResp, err := client.Do(headReq)
	if err != nil || headResp.ContentLength < 1 {
		return 0, fmt.Errorf("unable to get content length via HEAD: %w", err)
	}

	return headResp.ContentLength, err
}

// GetContentLengthViaGET returns the content length of the given URL, using the given HTTPClient. It
// uses a GET request with a zeroed Range header to get the content length.
func GetContentLengthViaGET(url *url.URL, client HTTPClient) (int64, error) {
	lengthReq, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return 0, err
	}
	lengthReq.Header.Set("Range", ByteRange{From: 0, To: 0}.RangeHeader())
	lengthResp, err := client.Do(lengthReq)
	if err != nil {
		return 0, err
	}
	contentRangeHeaderParts := strings.Split(lengthResp.Header.Get("Content-Range"), "/")
	if len(contentRangeHeaderParts) < 2 {
		return 0, errors.New("could not figure out content length")
	}
	return strconv.ParseInt(contentRangeHeaderParts[1], 10, 64)
}

// GetContentLength returns the content length of the given URL, using the given HTTPClient. It first
// attempts to use the HEAD method, but if that fails, falls back to using the GET method.
func GetContentLength(url *url.URL, client HTTPClient) (int64, error) {
	headLength, headErr := GetContentLengthViaHEAD(url, client)
	if headErr != nil {
		length, getErr := GetContentLengthViaGET(url, client)
		if getErr != nil {
			wrapErr := fmt.Errorf("error getting content length via HEAD: %w", headErr)
			return 0, fmt.Errorf("error getting content length via HEAD and GET: %w", wrapErr)
		}
		return length, nil
	}
	return headLength, nil
}

// NewRangingHTTPClient wraps and uses the given HTTPClient to make requests only
// for chunks designated by the given Ranger. This is useful for downloading large-files from
// cache-friendly sources in manageable chunks.
func NewRangingHTTPClient(ranger Ranger, client HTTPClient) RangingHTTPClient {
	return NewParallelRangingClient(ranger, client, 1)
}

// NewParallelRangingClient wraps and uses the given HTTPClient to make requests only
// for chunks designated by the given Ranger, but does so in parallel with the given
// number of goroutines. This is useful for downloading large files from
// cache-friendly sources in manageable chunks, with the added speed benefits of parallelism.
func NewParallelRangingClient(ranger Ranger, client HTTPClient, parallelism int) RangingHTTPClient {
	return RangingHTTPClient{
		ranger:      ranger,
		client:      client,
		parallelism: parallelism,
	}
}
