package ranger

import (
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
	err = s.prepare()
	if err != nil {
		return 0, err
	}

	n, err = s.current.Read(p)
	s.offset += int64(n)
	if err == io.EOF { // we have read all the bytes from the current range, close it
		_ = s.Close()
		s.current = nil // the next Read will open the next range
	}
	if err == io.EOF && s.offset < s.ranger.Length() { // we have more ranges to read, don't send EOF
		err = nil
	}

	return n, err
}

func (s *seqReader) prepare() error {
	if s.current == nil {
		br := s.ranger.At(s.offset)
		resp, err := s.makeRangeRequest(br)
		if err != nil {
			return err
		}
		s.current = resp.Body
	}
	return nil
}

func (s *seqReader) makeRangeRequest(br ByteRange) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, s.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", br.RangeHeader())
	return s.client.Do(req)
}

func (s *seqReader) Seek(offset int64, whence int) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (s *seqReader) Close() error {
	if s.current != nil {
		return s.current.Close()
	}
	return nil
}

func NewSeqReader(client *http.Client, url string, ranger SizedRanger) io.ReadSeekCloser {
	return &seqReader{
		url:    url,
		ranger: ranger,
		offset: 0,
		client: client,
	}
}
