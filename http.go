package ranger

import (
	"net/http"
)

// HTTPClient provides an interface allowing us to perform HTTP requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// RangingHTTPClient wraps another HTTP client to issue all requests based on the ranges provided.
type RangingHTTPClient struct {
	client HTTPClient
	ranger Ranger
	HTTPClient
}

func (rhc RangingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	headReq, err := http.NewRequest("HEAD", req.URL.String(), nil)
	noErr(err)
	headResp, err := rhc.client.Do(headReq)
	noErr(err)
	ranges := rhc.ranger.Ranges(headResp.ContentLength, 0)
	cr := NewChannellingReader()

	go func() {
		for _, r := range ranges {
			partReq, err := http.NewRequest("GET", req.URL.String(), nil)
			partReq.Header.Set("Range", r.Header())
			partResp, err := rhc.client.Do(partReq)
			noErr(err)
			cr.Send(partResp.Body)
		}
		cr.Finish()
	}()

	combinedResponse := &http.Response{
		Status:           "200 OK",
		StatusCode:       200,
		Proto:            "",
		ProtoMajor:       0,
		ProtoMinor:       0,
		Header:           nil,
		Body:             cr,
		ContentLength:    headResp.ContentLength,
		TransferEncoding: nil,
		Close:            false,
		Uncompressed:     false,
		Trailer:          nil,
		Request:          req,
		TLS:              nil,
	}
	noErr(err)
	return combinedResponse, nil

}

func NewRangingHTTPClient(ranger Ranger, client HTTPClient) RangingHTTPClient {
	return RangingHTTPClient{
		ranger: ranger,
		client: client,
	}
}

func noErr(err error) {
	if err != nil {
		panic(err)
	}
}
