package ranger

import (
	"io"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReader(t *testing.T) {
	ranger := NewRanger(2)
	data := makeData(10)
	rf := NewRangedSource(int64(len(data)), LoaderFunc(func(br ByteRange) ([]byte, error) {
		return data[br.From : br.To+1], nil
	}), ranger)
	pr := rf.Reader(3)
	received, err := io.ReadAll(pr)
	assert.NoError(t, err)
	assert.Equal(t, data, received)
}

func TestReaderOffset(t *testing.T) {
	ranger := NewRanger(2)
	data := makeData(10)
	rf := NewRangedSource(int64(len(data)), LoaderFunc(func(br ByteRange) ([]byte, error) {
		return data[br.From : br.To+1], nil
	}), ranger)

	table := []struct {
		offset int64
		data   []byte
	}{
		{5, data[5:]},
		{6, data[6:]},
		{4, data[4:]},
		{3, data[3:]},
		{0, data},
		{9, data[9:]},
		{10, []byte{}},
	}
	for _, tc := range table {
		t.Run(strconv.Itoa(int(tc.offset)), func(t *testing.T) {
			pr := rf.Reader(3)
			_, err := pr.Seek(tc.offset, io.SeekStart)
			assert.NoError(t, err)
			received, err := io.ReadAll(pr)
			assert.NoError(t, err)
			assert.Equal(t, tc.data, received)
		})
	}
}

func TestReaderLeaks(t *testing.T) {
	ranger := NewRanger(2)
	data := makeData(10)
	rf := NewRangedSource(int64(len(data)), LoaderFunc(func(br ByteRange) ([]byte, error) {
		return data[br.From : br.To+1], nil
	}), ranger)
	pr := rf.Reader(2)
	received, err := io.ReadAll(io.LimitReader(pr, 4))
	assert.NoError(t, err)
	assert.Equal(t, data[:4], received)
	pr.(io.ReadCloser).Close()
}
