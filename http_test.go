package ranger

import (
	"bytes"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBasicDownload(t *testing.T) {
	rand.Seed(42)
	content := make([]byte, 10000)
	rand.Read(content)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.ServeContent(writer, request, "", time.Time{}, bytes.NewReader(content))
	}))
	rangerClient := NewRangingHTTPClient(NewRanger(1000), http.DefaultClient)
	req, err := http.NewRequest("GET", server.URL, nil)
	assert.Nil(t, err)
	response, err := rangerClient.Do(req)
	assert.Nil(t, err)
	servedContent, err := io.ReadAll(response.Body)
	assert.Nil(t, err)
	assert.Equal(t, content, servedContent)
}
