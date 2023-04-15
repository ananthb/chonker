package ranger

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HTTPClient provides an interface allowing us to perform HTTP requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type customResponseWriter struct {
	header http.Header
	pr     *io.PipeReader
	pw     *io.PipeWriter
}

func newCustomResponseWriter() *customResponseWriter {
	pr, pw := io.Pipe()
	return &customResponseWriter{
		header: http.Header{},
		pr:     pr,
		pw:     pw,
	}
}
func (w *customResponseWriter) Header() http.Header         { return w.header }
func (w *customResponseWriter) Write(b []byte) (int, error) { return w.pw.Write(b) }
func (w *customResponseWriter) WriteHeader(_ int)           {}

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
	reader := remoteFile.Reader(rhc.parallelism)

	crw := newCustomResponseWriter()
	go func() {
		http.ServeContent(crw, req, "", time.Time{}, reader)
		_ = crw.pw.Close()
	}()

	resp := &http.Response{
		Status:        http.StatusText(200),
		StatusCode:    200,
		Body:          crw.pr,
		ContentLength: contentLength,
		Request:       req,
	}

	return resp, nil
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

// NewRangingClient wraps and uses the given HTTPClient to make requests only
// for chunks designated by the given Ranger, but does so in parallel with the given
// number of goroutines. This is useful for downloading large files from
// cache-friendly sources in manageable chunks, with the added speed benefits of parallelism.
func NewRangingClient(ranger Ranger, client HTTPClient, parallelism int) RangingHTTPClient {
	return RangingHTTPClient{
		ranger:      ranger,
		client:      client,
		parallelism: parallelism,
	}
}
