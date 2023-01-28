package ranger

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasicDownload(t *testing.T) {
	content := makeData(10000)
	server := makeServer(t, content)

	rangerClient := NewRangingHTTPClient(NewRanger(1000), http.DefaultClient)

	req, err := http.NewRequest("GET", server.URL, nil)
	assert.Nil(t, err)
	response, err := rangerClient.Do(req)
	assert.Nil(t, err)
	servedContent, err := io.ReadAll(response.Body)
	assert.Nil(t, err)
	assert.Equal(t, content, servedContent)
}
