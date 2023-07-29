package ranger

import (
	"errors"
	"fmt"
	"github.com/gotd/contrib/http_range"
	"io"
	"net/http"
)

type seqReader struct {
	url     string
	ranger  SizedRanger
	offset  int64
	client  *http.Client
	current io.ReadCloser
}

func (s *seqReader) Read(p []byte) (n int, err error) {
	if s.offset >= s.ranger.Length() {
		return 0, io.EOF
	}

	err = s.prepare()
	if err != nil {
		return 0, err
	}

	n, err = s.current.Read(p)
	s.offset += int64(n)

	if err == io.EOF { // we have read all the bytes from the current range, close it
		s.reset()
		if s.offset < s.ranger.Length() {
			err = nil // we still have bytes to read, so we don't want to return EOF
		}
	}
	return n, err
}

func (s *seqReader) prepare() (err error) {
	if s.current != nil {
		return
	}
	br := s.ranger.At(s.offset).Floor(s.offset) // get the relevant byte range and start it from the current offset (needed for seek)
	resp, err := s.makeRangeRequest(br)
	if err == nil {
		s.current = resp.Body
	}
	return
}

func (s *seqReader) makeRangeRequest(br ByteRange) (resp *http.Response, err error) {
	req, err := br.Request(s.url)
	if err == nil {
		resp, err = s.client.Do(req)
	}
	return
}

func (s *seqReader) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = s.offset + offset
	case io.SeekEnd:
		newOffset = s.ranger.length + offset
	default:
		return 0, errors.New("invalid whence value")
	}

	if newOffset < 0 || newOffset > s.ranger.length {
		return 0, errors.New("seek out of bounds")
	}

	s.reset()
	s.offset = newOffset

	return newOffset, nil
}

func (s *seqReader) Close() error {
	if s.current != nil {
		return s.current.Close()
	}
	return nil
}

func (s *seqReader) reset() {
	if s.current != nil {
		_ = s.Close()
	}
	s.current = nil // the next Read will open the next range
}

// NewSeqReader returns a new io.ReadSeekCloser that reads from the given url using the given client. Instead of
// reading the whole file at once, it reads the file in sequential chunks, using the given ranger to determine the
// ranges to read. This allows for reading very large files in CDN-cacheable chunks using RANGE GETs.
func NewSeqReader(client *http.Client, url string, ranger SizedRanger) io.ReadSeekCloser {
	return &seqReader{
		url:    url,
		ranger: ranger,
		offset: 0,
		client: client,
	}
}

type seqRangingClient struct {
	ranger Ranger
	client *http.Client
}

func (s seqRangingClient) RoundTrip(request *http.Request) (*http.Response, error) {
	headReq, err := http.NewRequest("HEAD", request.URL.String(), nil)
	if err != nil {
		return nil, err
	}

	headResp, err := s.client.Do(headReq)
	if err != nil || headResp.ContentLength < 1 {
		return nil, fmt.Errorf("unable to get content length via HEAD: %w", err)
	}

	parseRange, err := http_range.ParseRange(request.Header.Get("Range"), headResp.ContentLength)
	if err != nil || len(parseRange) > 1 {
		return nil, fmt.Errorf("unable to parse Range header correctly: %w", err)
	}

	seqr := NewSeqReader(s.client, request.URL.String(), NewSizedRanger(headResp.ContentLength, s.ranger))
	fetchRange := http_range.Range{
		Start:  0,
		Length: headResp.ContentLength,
	}
	if parseRange != nil {
		fetchRange = parseRange[0]
	}
	_, err = seqr.Seek(fetchRange.Start, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("unable to seek correctly: %w", err)
	}

	return &http.Response{
		Status:        http.StatusText(http.StatusOK),
		StatusCode:    http.StatusOK,
		Proto:         headResp.Proto,
		ProtoMajor:    headResp.ProtoMajor,
		ProtoMinor:    headResp.ProtoMinor,
		Header:        headResp.Header,
		Body:          readCloser{io.LimitReader(seqr, fetchRange.Length), seqr},
		ContentLength: fetchRange.Length,
		Close:         true,
		Request:       request,
		TLS:           headResp.TLS,
	}, nil
}

func NewSeqRangingClient(ranger Ranger, client *http.Client) http.RoundTripper {
	return &seqRangingClient{
		ranger: ranger,
		client: client,
	}
}

type readCloser struct {
	r io.Reader
	c io.Closer
}

func (rc readCloser) Read(p []byte) (n int, err error) {
	return rc.r.Read(p)
}

func (rc readCloser) Close() error {
	return rc.c.Close()
}
