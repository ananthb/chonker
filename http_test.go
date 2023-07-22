package ranger

import (
	"bytes"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBasicDownload(t *testing.T) {
	content := makeData(1000)
	server := makeHTTPServer(t, content)

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
	server := makeHTTPServer(t, content)

	testTable := []struct {
		rangeHeader string
		expected    []byte
	}{
		{rangeHeader: "bytes=0-1", expected: content[0:2]},
		{rangeHeader: "bytes=9-", expected: content[9:]},
		{rangeHeader: "bytes=5-", expected: content[5:]},
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

func testClients() []*http.Client {
	return []*http.Client{
		{Transport: NewRangingClient(NewRanger(100), http.DefaultClient, 3)},
		{Transport: NewRangingClient(NewRanger(10), http.DefaultClient, 3)},
		{Transport: NewRangingClient(NewRanger(10000), http.DefaultClient, 3)},
		{Transport: NewRangingClient(NewRanger(1024), http.DefaultClient, 1)},
		{Transport: NewRangingClient(NewRanger(100), http.DefaultClient, 0)},
		http.DefaultClient,
	}
}

func makeHTTPServer(t *testing.T, content []byte) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		time.Sleep(time.Duration(rand.Intn(1000)) * time.Microsecond)
		http.ServeContent(writer, request, "", time.Time{}, bytes.NewReader(content))
	}))
	return server
}
