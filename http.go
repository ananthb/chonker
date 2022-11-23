package ranger

import (
	"bytes"
	"io"
	"net/http"
)

// HTTPClient provides an interface allowing us to perform HTTP requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// RangingHTTPClient wraps another HTTP client to issue all requests based on the Ranges provided.
type RangingHTTPClient struct {
	client HTTPClient
	ranger Ranger
	HTTPClient
}

func (rhc RangingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	contentLength, err := rhc.getContentLength(req)
	panicIfErr(err)
	rangedReader := rhc.ranger.RangedReader(contentLength, 0, func(br ByteRange) io.Reader {
		partReq, err := http.NewRequest("GET", req.URL.String(), nil)
		panicIfErr(err)
		partReq.Header.Set("Range", br.Header())
		partResp, err := rhc.client.Do(partReq)
		panicIfErr(err)
		buf := bytes.NewBuffer(make([]byte, 0, br.Length()))
		_, err = buf.ReadFrom(partResp.Body)
		panicIfErr(err)
		err = partResp.Body.Close()
		panicIfErr(err)
		return buf
	})

	combinedResponse := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Body:          io.NopCloser(rangedReader),
		ContentLength: contentLength,
		Request:       req,
	}
	panicIfErr(err)

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

func NewRangingHTTPClient(ranger Ranger, client HTTPClient) RangingHTTPClient {
	return RangingHTTPClient{
		ranger: ranger,
		client: client,
	}
}
