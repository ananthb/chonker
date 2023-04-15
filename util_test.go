package ranger

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func makeData(size int) []byte {
	rnd := rand.New(rand.NewSource(42))
	content := make([]byte, size)
	rnd.Read(content)
	return content
}

func Test_min(t *testing.T) {
	assert.True(t, min(1, 2) == 1)
	assert.True(t, min(0, 2) == 0)
	assert.True(t, min(-1, 2) == -1)
	assert.True(t, min(2, 2) == 2)
}

func Test_max(t *testing.T) {
	assert.True(t, max(1, 2) == 2)
	assert.True(t, max(2, 1) == 2)
	assert.True(t, max(0, 2) == 2)
	assert.True(t, max(-2, 2) == 2)
	assert.True(t, max(2, 2) == 2)
}
