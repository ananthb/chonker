package ranger

import (
	"bytes"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func makeData(size int) []byte {
	rnd := rand.New(rand.NewSource(42))
	content := make([]byte, size)
	rnd.Read(content)
	return content
}

func makeHTTPServer(t *testing.T, content []byte) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.ServeContent(writer, request, "", time.Now(), bytes.NewReader(content))
	}))
	return server
}
