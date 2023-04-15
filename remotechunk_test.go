package ranger

import (
	"io"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReader(t *testing.T) {
	data, rf := createTestData()
	pr := rf.Reader(3)
	received, err := io.ReadAll(pr)
	assert.NoError(t, err)
	assert.Equal(t, data, received)
}

func TestReaderOffset(t *testing.T) {
	data, rf := createTestData()

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

func TestRangedReadSeekCloser_Seek(t *testing.T) {
	_, rf := createTestData()
	pr := rf.Reader(3)
	table := []struct {
		offset         int64
		whence         int
		expectedOffset int64
	}{
		{0, io.SeekStart, 0},
		{5, io.SeekStart, 5},
		{0, io.SeekEnd, 10},
		{-1, io.SeekEnd, 9},
		{-2, io.SeekEnd, 8},
		{0, io.SeekCurrent, 8},
		{-1, io.SeekCurrent, 7},
		{2, io.SeekCurrent, 9},
	}
	for _, tc := range table {
		t.Run(strconv.Itoa(int(tc.offset))+"/"+strconv.Itoa(int(tc.whence)), func(t *testing.T) {
			newOffset, err := pr.Seek(tc.offset, tc.whence)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedOffset, newOffset)
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

func createTestData() ([]byte, RangedSource) {
	ranger := NewRanger(2)
	data := makeData(10)
	rf := NewRangedSource(int64(len(data)), LoaderFunc(func(br ByteRange) ([]byte, error) {
		return data[br.From : br.To+1], nil
	}), ranger)
	return data, rf
}
