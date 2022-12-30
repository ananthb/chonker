package ranger

import (
	"bytes"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBasicDownload(t *testing.T) {
	content := makeSlice()
	server := makeServer(t, content)

	rangerClient := NewRangingHTTPClient(NewRanger(1000), http.DefaultClient, 10)

	req, err := http.NewRequest("GET", server.URL, nil)
	assert.Nil(t, err)
	response, err := rangerClient.Do(req)
	assert.Nil(t, err)
	servedContent, err := io.ReadAll(response.Body)
	assert.Nil(t, err)
	assert.Equal(t, content, servedContent)
}

func makeSlice() []byte {
	rand.Seed(42)
	content := make([]byte, 10000)
	rand.Read(content)
	return content
}

func makeServer(t *testing.T, content []byte) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		dumpRequest, _ := httputil.DumpRequest(request, false)
		t.Log(time.Now(), string(dumpRequest))
		http.ServeContent(writer, request, "", time.Time{}, bytes.NewReader(content))
	}))
	return server
}
