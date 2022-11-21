package ranger

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"

	"github.com/sudhirj/cirque"
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
	rangeInputs, readerOutputs := cirque.NewCirque(2, func(br byteRange) io.Reader {
		partReq, err := http.NewRequest("GET", req.URL.String(), nil)
		panicIfErr(err)
		partReq.Header.Set("Range", br.Header())
		partResp, err := rhc.client.Do(partReq)
		panicIfErr(err)
		buf := new(bytes.Buffer)
		buf.Grow(br.length())
		_, err = buf.ReadFrom(partResp.Body)
		panicIfErr(err)
		err = partResp.Body.Close()
		panicIfErr(err)
		return buf
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
		Status:        "200 OK",
		StatusCode:    200,
		Body:          combinedReader,
		ContentLength: headResp.ContentLength,
		Request:       req,
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
