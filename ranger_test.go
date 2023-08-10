package ranger

import (
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

func TestIndex(t *testing.T) {
	ranger := NewRanger(10)
	assert.Equal(t, 0, ranger.Index(0))
	assert.Equal(t, 0, ranger.Index(5))
	assert.Equal(t, 0, ranger.Index(9))
	assert.Equal(t, 1, ranger.Index(10))
	assert.Equal(t, 4, ranger.Index(42))
	assert.Equal(t, 9, ranger.Index(99))
}

func TestSizedRanger_RangeContaining(t *testing.T) {
	r := NewRanger(10)
	sr := NewSizedRanger(100, r)
	assert.Equal(t, ByteRange{From: 0, To: 9}, sr.RangeContaining(0))
	assert.Equal(t, ByteRange{From: 90, To: 99}, sr.RangeContaining(95))
	assert.Equal(t, ByteRange{From: 0, To: 0}, sr.RangeContaining(150))
}
