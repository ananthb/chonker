package ranger

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
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

	loader := LoaderFunc(func(br ByteRange) ([]byte, error) {
		partReq, err := http.NewRequest("GET", req.URL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("error building GET request for segment %v: %w", br.RangeHeader(), err)
		}

		partReq.Header.Set("Range", br.RangeHeader())
		partResp, err := rhc.client.Do(partReq)
		if err != nil {
			return nil, fmt.Errorf("error making the request for segment %v: %w", br.RangeHeader(), err)
		}

		buf := bytes.NewBuffer(make([]byte, 0, br.Length()))
		n, err := buf.ReadFrom(partResp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading the request for segment %v: %w", br.RangeHeader(), err)
		}

		if n != br.Length() {
			return nil, fmt.Errorf("error with received byte count on segment %v: expected %v bytes, but received %v", br.RangeHeader(), br.Length(), n)
		}

		err = partResp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("error closing the request for segment %v: %w", br.RangeHeader(), err)
		}

		return buf.Bytes(), nil
	})

	loaderWithLRUCache := WrapLoaderWithLRUCache(loader, 3)
	loaderWithSingleFlight := WrapLoaderWithSingleFlight(loaderWithLRUCache)
	remoteFile := NewRangedSource(contentLength, loaderWithSingleFlight, rhc.ranger)

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
	return headResp.ContentLength, err
}

func NewRangingHTTPClient(ranger Ranger, client HTTPClient, parallelism int) RangingHTTPClient {
	return RangingHTTPClient{
		ranger:      ranger,
		client:      client,
		parallelism: parallelism,
	}
}
