package ranger

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Loader implements a Load method that provides data as byte slice for a
// given byte range chunk.
//
// `Load` should be safe to call from multiple goroutines.
//
// If err is nil, the returned byte slice must always have exactly as many bytes as was
// asked for, i.e. `len([]byte)` returned must always be equal to `br.Ranges()`.
type Loader interface {
	Load(br ByteRange) ([]byte, error)
}

// LoaderFunc converts a Load function into a Loader type.
type LoaderFunc func(br ByteRange) ([]byte, error)

func (l LoaderFunc) Load(br ByteRange) ([]byte, error) {
	return l(br)
}

func HTTPLoader(url *url.URL, client HTTPClient) Loader {
	return LoaderFunc(func(br ByteRange) ([]byte, error) {
		partReq, err := http.NewRequest("GET", url.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("error building GET request for segment %v: %w", br.RangeHeader(), err)
		}

		partReq.Header.Set("Range", br.RangeHeader())

		partResp, err := client.Do(partReq)
		if err != nil {
			return nil, fmt.Errorf("error making the request for segment %v: %w", br.RangeHeader(), err)
		}

		data, err := io.ReadAll(partResp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading data for segment %v: %w", br.RangeHeader(), err)
		}

		_ = partResp.Body.Close()
		return data, nil

	})
}

func DefaultHTTPLoader(url *url.URL) Loader {
	return HTTPLoader(url, http.DefaultClient)
}
