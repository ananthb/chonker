package ranger

import (
	"testing"

	"github.com/dsnet/golib/memfile"
	"github.com/stretchr/testify/assert"
)

func TestParallelWriter(t *testing.T) {
	data := makeData(100)
	pw := NewParallelWriter(
		int64(len(data)),
		LoaderFunc(func(br ByteRange) ([]byte, error) {
			return data[br.From : br.To+1], nil
		}), NewRanger(7))
	file := memfile.New(nil)
	err := pw.WriteInto(file, 10)
	assert.NoError(t, err)
	assert.Equal(t, data, file.Bytes())
}
