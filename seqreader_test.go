package ranger

import (
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"testing/iotest"
)

func TestSeqReader(t *testing.T) {
	content := makeData(1000)
	server := makeHTTPServer(t, content)

	for _, size := range []int{1, 3, 512, 1024, 2048} {
		ranger := NewSizedRanger(int64(len(content)), NewRanger(int64(size)))
		seqr := NewSeqReader(http.DefaultClient, server.URL, ranger)

		assert.NoError(t, iotest.TestReader(seqr, content))
	}
}

func TestSeqHTTPClient(t *testing.T) {
	content := makeData(100)
	server := makeHTTPServer(t, content)

	postServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		err := request.ParseForm()
		assert.Equal(t, "world", request.Form.Get("hello"))
		assert.NoError(t, err)
		assert.NotEqual(t, request.Method, http.MethodGet)
	}))

	clients := []*http.Client{
		http.DefaultClient, // all clients must behave the same as the default HTTP client
		{Transport: NewSeqRangingClient(NewRanger(10), http.DefaultClient)},
		{Transport: NewSeqRangingClient(NewRanger(1000), http.DefaultClient)},
	}
	testCases := []struct {
		rangeHeader string
		expected    []byte
	}{
		{rangeHeader: "", expected: content[:]},
		{rangeHeader: "bytes=42-", expected: content[42:]},
		{rangeHeader: "bytes=42-84", expected: content[42:85]},
	}

	for clientIndex, client := range clients {
		for _, testCase := range testCases {
			t.Run("Client["+strconv.Itoa(clientIndex)+"]:"+testCase.rangeHeader, func(t *testing.T) {
				req, err := http.NewRequest("GET", server.URL, nil)
				assert.NoError(t, err)
				if testCase.rangeHeader != "" {
					req.Header.Set("Range", testCase.rangeHeader)
				}
				response, err := client.Do(req)
				assert.NoError(t, err)
				servedContent, err := io.ReadAll(response.Body)
				assert.NoError(t, err)
				assert.Equal(t, testCase.expected, servedContent)

				_, err = client.PostForm(postServer.URL, map[string][]string{"hello": {"world"}})
				assert.NoError(t, err)
			})
		}
	}
}
