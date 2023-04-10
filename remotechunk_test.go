package ranger

import (
	"context"
	"io"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParallelReader(t *testing.T) {
	ranger := NewRanger(2)
	data := makeData(10)
	rf := NewRangedSource(int64(len(data)), LoaderFunc(func(br ByteRange) ([]byte, error) {
		//t.Log("loading", time.Now(), br)
		//time.Sleep(100 * time.Millisecond)
		return data[br.From : br.To+1], nil
	}), ranger)
	pr := rf.ParallelReader(context.Background(), 3)
	received, err := io.ReadAll(pr)
	assert.NoError(t, err)
	assert.Equal(t, data, received)
}

func TestParallelOffsetReader(t *testing.T) {
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
		{4, data[4:]},
		{0, data},
		{9, data[9:]},
		{10, []byte{}},
	}
	for _, tc := range table {
		t.Run(strconv.Itoa(int(tc.offset)), func(t *testing.T) {
			pr := rf.ParallelOffsetReader(context.Background(), 3, tc.offset)
			received, err := io.ReadAll(pr)
			assert.NoError(t, err)
			assert.Equal(t, tc.data, received)
		})
	}
}

func TestParallelReaderLeaks(t *testing.T) {
	ranger := NewRanger(2)
	data := makeData(10)
	rf := NewRangedSource(int64(len(data)), LoaderFunc(func(br ByteRange) ([]byte, error) {
		return data[br.From : br.To+1], nil
	}), ranger)
	pr := rf.ParallelReader(context.Background(), 2)
	received, err := io.ReadAll(io.LimitReader(pr, 4))
	assert.NoError(t, err)
	assert.Equal(t, data[:4], received)
	pr.(io.ReadCloser).Close()
}
