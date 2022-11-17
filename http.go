package ranger

import (
	"github.com/sudhirj/cirque"
	"io"
	"net/http"
	"net/http/httputil"
)

// HTTPClient provides an interface allowing us to perform HTTP requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// RangingHTTPClient wraps another HTTP client to issue all requests based on the ranges provided.
type RangingHTTPClient struct {
	client HTTPClient
	ranger Ranger
	httputil.BufferPool
	HTTPClient
}

func (rhc RangingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	headReq, err := http.NewRequest("HEAD", req.URL.String(), nil)
	panicIfErr(err)
	headResp, err := rhc.client.Do(headReq)
	panicIfErr(err)
	ranges := rhc.ranger.ranges(headResp.ContentLength, 0)
	combinedReader := NewChannellingReader()
	rangeInputs, readerOutputs := cirque.NewCirque(2, func(i byteRange) io.ReadCloser {
		partReq, err := http.NewRequest("GET", req.URL.String(), nil)
		panicIfErr(err)
		partReq.Header.Set("Range", i.Header())
		partResp, err := rhc.client.Do(partReq)
		panicIfErr(err)
		return partResp.Body
	})

	go func() {
		for r := range readerOutputs {
			combinedReader.WriteFrom(r)
		}
		combinedReader.FinishWriting()
	}()

	go func() {
		for _, br := range ranges {
			rangeInputs <- br
		}
		close(rangeInputs)
	}()

	combinedResponse := &http.Response{
		Status:           "200 OK",
		StatusCode:       200,
		Proto:            "",
		ProtoMajor:       0,
		ProtoMinor:       0,
		Header:           nil,
		Body:             combinedReader,
		ContentLength:    headResp.ContentLength,
		TransferEncoding: nil,
		Close:            false,
		Uncompressed:     false,
		Trailer:          nil,
		Request:          req,
		TLS:              nil,
	}
	panicIfErr(err)

	return combinedResponse, nil
}

func NewRangingHTTPClient(ranger Ranger, client HTTPClient) RangingHTTPClient {
	return RangingHTTPClient{
		ranger: ranger,
		client: client,
	}
}
