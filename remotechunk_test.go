package ranger

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadAt(t *testing.T) {
	ranger := NewRanger(3)
	data := makeData(10)
	rf := NewRangedSource(10, LoaderFunc(func(br ByteRange) ([]byte, error) {
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
	rf := NewRangedSource(10, LoaderFunc(func(br ByteRange) ([]byte, error) {
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
