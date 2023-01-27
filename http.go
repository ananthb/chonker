package ranger

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// HTTPClient provides an interface allowing us to perform HTTP requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// RangingHTTPClient wraps another HTTP client to issue all requests based on the Ranges provided.
type RangingHTTPClient struct {
	client      HTTPClient
	ranger      Ranger
	parallelism int
	HTTPClient
}

func (rhc RangingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	contentLength, err := rhc.getContentLength(req)
	if err != nil {
		return nil, fmt.Errorf("error getting content length via HEAD: %w", err)
	}
	log.Println("content length", contentLength)
	loader := SingleFlightLoaderWrap(HTTPLoader(req.URL, rhc.client))

	loader = SingleFlightLoaderWrap(loader)
	loader = SingleFlightLoaderWrap(LRUCacheLoaderWrap(loader, 10))

	remoteFile := NewRangedSource(contentLength, loader, rhc.ranger)

	combinedResponse := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Body:          io.NopCloser(remoteFile.Reader()),
		ContentLength: contentLength,
		Request:       req,
	}

	return combinedResponse, nil
}

func GetContentLengthFromHEAD(url *url.URL, client HTTPClient) (int64, error) {
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

func GetContentLengthFromGET(url *url.URL, client HTTPClient) (int64, error) {
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
	log.Println("headers", lengthResp.Header)
	return strconv.ParseInt(contentRangeHeaderParts[1], 10, 64)
}

func GetContentLength(url *url.URL, client HTTPClient) (int64, error) {
	headLength, headErr := GetContentLengthFromHEAD(url, client)
	if headErr != nil {
		length, getErr := GetContentLengthFromGET(url, client)
		if getErr != nil {
			wrapErr := fmt.Errorf("error getting content length via HEAD: %w", headErr)
			return 0, fmt.Errorf("error getting content length via HEAD and GET: %w", wrapErr)
		}
		return length, nil
	}
	return headLength, nil
}
func (rhc RangingHTTPClient) getContentLength(req *http.Request) (int64, error) {
	return GetContentLength(req.URL, rhc.client)
}

func NewRangingHTTPClient(ranger Ranger, client HTTPClient, parallelism int) RangingHTTPClient {
	return RangingHTTPClient{
		ranger:      ranger,
		client:      client,
		parallelism: parallelism,
	}
}
