package ranger

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
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
	loader := SingleFlightLoaderWrap(LoaderFunc(func(br ByteRange) ([]byte, error) {
		log.Println("ACTUAL.REQUEST", br)
		partReq, err := http.NewRequest("GET", req.URL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("error building GET request for segment %v: %w", br.RangeHeader(), err)
		}

		partReq.Header.Set("Range", br.RangeHeader())
		partResp, err := rhc.client.Do(partReq)
		if err != nil {
			return nil, fmt.Errorf("error making the request for segment %v: %w", br.RangeHeader(), err)
		}

		buf, err := io.ReadAll(partResp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading data for segment %v: %w", br.RangeHeader(), err)
		}

		err = partResp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("error closing the request for segment %v: %w", br.RangeHeader(), err)
		}
		return buf, nil
	}))

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

func (rhc RangingHTTPClient) getContentLength(req *http.Request) (int64, error) {
	headReq, err := http.NewRequest("HEAD", req.URL.String(), nil)
	if err != nil {
		return 0, err
	}
	headResp, err := rhc.client.Do(headReq)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}
	if errors.Is(err, io.EOF) || headResp.ContentLength < 1 {
		return rhc.getContentLengthWithGETRequest(req)
	}

	log.Println("head", headResp.Header)
	return headResp.ContentLength, err
}

func (rhc RangingHTTPClient) getContentLengthWithGETRequest(req *http.Request) (int64, error) {
	lengthReq, err := http.NewRequest("GET", req.URL.String(), nil)
	if err != nil {
		return 0, err
	}
	lengthReq.Header.Set("Range", ByteRange{From: 0, To: 0}.RangeHeader())
	lengthResp, err := rhc.client.Do(lengthReq)
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

func NewRangingHTTPClient(ranger Ranger, client HTTPClient, parallelism int) RangingHTTPClient {
	return RangingHTTPClient{
		ranger:      ranger,
		client:      client,
		parallelism: parallelism,
	}
}
