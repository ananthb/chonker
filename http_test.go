package ranger

import (
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasicDownload(t *testing.T) {
	content := makeData(10000)
	server := makeServer(t, content)

	for clientIndex, client := range testClients() {
		t.Run("client:"+strconv.Itoa(clientIndex+1), func(t *testing.T) {
			req, err := http.NewRequest("GET", server.URL, nil)
			assert.Nil(t, err)
			response, err := client.Do(req)
			assert.Nil(t, err)
			servedContent, err := io.ReadAll(response.Body)
			assert.Nil(t, err)
			assert.Equal(t, content, servedContent)
		})
	}

}

func TestOffsetDownload(t *testing.T) {
	content := makeData(10)
	server := makeServer(t, content)

	testTable := []struct {
		rangeHeader string
		expected    []byte
	}{
		{
			rangeHeader: "bytes=0-1",
			expected:    content[0:2],
		},
		{
			rangeHeader: "bytes=5-",
			expected:    content[5:],
		},
	}
	for _, testCase := range testTable {
		for clientIndex, client := range testClients() {
			t.Run("client:"+strconv.Itoa(clientIndex+1)+"/"+testCase.rangeHeader, func(t *testing.T) {
				req, err := http.NewRequest("GET", server.URL, nil)
				assert.Nil(t, err)
				req.Header.Set("Range", testCase.rangeHeader)
				response, err := client.Do(req)
				assert.Nil(t, err)
				servedContent, err := io.ReadAll(response.Body)
				assert.Nil(t, err)
				assert.Equal(t, testCase.expected, servedContent)
			})
		}

	}

}

func testClients() []HTTPClient {
	return []HTTPClient{
		NewRangingHTTPClient(NewRanger(1000), http.DefaultClient),
		NewParallelRangingClient(NewRanger(1000), http.DefaultClient, 3),
		NewParallelRangingClient(NewRanger(1000), http.DefaultClient, 1),
		http.DefaultClient,
	}
}
