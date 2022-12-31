package ranger

import (
	"io"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleRanger(t *testing.T) {
	ranger := NewRanger(10)
	ranges := ranger.Ranges(100)
	assert.Equal(t, 10, len(ranges))
	assert.Equal(t, ByteRange{From: 0, To: 9}, ranges[0])
	assert.Equal(t, ByteRange{From: 10, To: 19}, ranges[1])
	assert.Equal(t, ByteRange{From: 20, To: 29}, ranges[2])
	assert.Equal(t, ByteRange{From: 30, To: 39}, ranges[3])
	assert.Equal(t, ByteRange{From: 40, To: 49}, ranges[4])
	assert.Equal(t, ByteRange{From: 50, To: 59}, ranges[5])
	assert.Equal(t, ByteRange{From: 60, To: 69}, ranges[6])
	assert.Equal(t, ByteRange{From: 70, To: 79}, ranges[7])
	assert.Equal(t, ByteRange{From: 80, To: 89}, ranges[8])
	assert.Equal(t, ByteRange{From: 90, To: 99}, ranges[9])
}

func TestOvershoot(t *testing.T) {
	ranger := NewRanger(75)
	ranges := ranger.Ranges(100)
	assert.Equal(t, 2, len(ranges))
	assert.Equal(t, ByteRange{From: 0, To: 74}, ranges[0])
	assert.Equal(t, ByteRange{From: 75, To: 99}, ranges[1])
}

func TestHeader(t *testing.T) {
	// Needs to use the format `bytes=0-50`
	ranger := NewRanger(10)
	ranges := ranger.Ranges(100)
	assert.Equal(t, ByteRange{From: 0, To: 9}, ranges[0])
	assert.Equal(t, "bytes=0-9", ranges[0].Header())
	assert.Equal(t, ByteRange{From: 10, To: 19}, ranges[1])
	assert.Equal(t, "bytes=10-19", ranges[1].Header())
}

func TestIndex(t *testing.T) {
	ranger := NewRanger(10)
	assert.Equal(t, 0, ranger.Index(0))
	assert.Equal(t, 0, ranger.Index(5))
	assert.Equal(t, 0, ranger.Index(9))
	assert.Equal(t, 1, ranger.Index(10))
	assert.Equal(t, 4, ranger.Index(42))
	assert.Equal(t, 9, ranger.Index(99))
}

func TestReadAt(t *testing.T) {
	ranger := NewRanger(3)
	data := makeData(10)
	rf := NewRemoteFile(10, LoaderFunc(func(br ByteRange) ([]byte, error) {
		return data[br.From : br.To+1], nil
	}), ranger)
	holder := make([]byte, 3)

	n, err := rf.ReadAt(holder, 0)
	assert.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, data[0:3], holder)

	n, err = rf.ReadAt(holder, 5)
	assert.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, data[5:8], holder)

	n, err = rf.ReadAt(holder, 9)
	assert.EqualError(t, err, io.EOF.Error())
	assert.Equal(t, 1, n)
	assert.Equal(t, data[9:10], holder[0:1])
}

func TestReadAtExtremes(t *testing.T) {
	ranger := NewRanger(3)
	data := makeData(10)
	rf := NewRemoteFile(10, LoaderFunc(func(br ByteRange) ([]byte, error) {
		return data[br.From : br.To+1], nil
	}), ranger)
	holder := make([]byte, 1)

	n, err := rf.ReadAt(holder, 0)
	assert.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, data[0:1], holder)

	n, err = rf.ReadAt(holder, 9)
	assert.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, data[9:10], holder)

	giantHolder := make([]byte, 1000)
	n, err = rf.ReadAt(giantHolder, 0)
	assert.EqualError(t, err, io.EOF.Error())
	assert.Equal(t, 10, n)
	assert.Equal(t, data, giantHolder[0:10])
}

func makeData(size int) []byte {
	rand.Seed(42)
	content := make([]byte, size)
	rand.Read(content)
	return content
}
