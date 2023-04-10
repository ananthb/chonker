package ranger

import (
	"bytes"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func makeData(size int) []byte {
	rand.Seed(42)
	content := make([]byte, size)
	rand.Read(content)
	return content
}

func makeServer(t *testing.T, content []byte) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		time.Sleep(time.Duration(rand.Intn(1000)) * time.Microsecond)
		http.ServeContent(writer, request, "", time.Time{}, bytes.NewReader(content))
	}))
	return server
}

func Test_min(t *testing.T) {
	assert.True(t, min(1, 2) == 1)
	assert.True(t, min(0, 2) == 0)
	assert.True(t, min(-1, 2) == -1)
	assert.True(t, min(2, 2) == 2)
}

func Test_max(t *testing.T) {
	assert.True(t, max(1, 2) == 2)
	assert.True(t, max(0, 2) == 2)
	assert.True(t, max(-2, 2) == 2)
	assert.True(t, max(2, 2) == 2)
}
