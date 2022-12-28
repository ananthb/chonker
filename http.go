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

	loader := func(br ByteRange) (io.Reader, error) {
		partReq, err := http.NewRequest("GET", req.URL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("error building GET request for segment %v: %w", br.Header(), err)
		}

		partReq.Header.Set("Range", br.Header())
		partResp, err := rhc.client.Do(partReq)
		if err != nil {
			return nil, fmt.Errorf("error making the request for segment %v: %w", br.Header(), err)
		}

		buf := bytes.NewBuffer(make([]byte, 0, br.Length()))
		n, err := buf.ReadFrom(partResp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading the request for segment %v: %w", br.Header(), err)
		}

		if n != br.Length() {
			return nil, fmt.Errorf("error with received byte count on segment %v: expected %v bytes, but received %v", br.Header(), br.Length(), n)
		}

		err = partResp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("error closing the request for segment %v: %w", br.Header(), err)
		}

		return buf, nil
	}

	rangedReader := NewChunkAlignedRemoteFile(contentLength, LoaderFunc(loader), rhc.ranger)

	combinedResponse := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Body:          io.NopCloser(rangedReader),
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
