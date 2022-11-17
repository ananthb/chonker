package ranger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleRanger(t *testing.T) {
	ranger := NewRanger(10)
	ranges := ranger.ranges(100, 0)
	assert.Equal(t, 10, len(ranges))
	assert.Equal(t, byteRange{from: 0, to: 9}, ranges[0])
	assert.Equal(t, byteRange{from: 10, to: 19}, ranges[1])
	assert.Equal(t, byteRange{from: 20, to: 29}, ranges[2])
	assert.Equal(t, byteRange{from: 30, to: 39}, ranges[3])
	assert.Equal(t, byteRange{from: 40, to: 49}, ranges[4])
	assert.Equal(t, byteRange{from: 50, to: 59}, ranges[5])
	assert.Equal(t, byteRange{from: 60, to: 69}, ranges[6])
	assert.Equal(t, byteRange{from: 70, to: 79}, ranges[7])
	assert.Equal(t, byteRange{from: 80, to: 89}, ranges[8])
	assert.Equal(t, byteRange{from: 90, to: 99}, ranges[9])
}

func TestOvershoot(t *testing.T) {
	ranger := NewRanger(75)
	ranges := ranger.ranges(100, 0)
	assert.Equal(t, 2, len(ranges))
	assert.Equal(t, byteRange{from: 0, to: 74}, ranges[0])
	assert.Equal(t, byteRange{from: 75, to: 99}, ranges[1])
}

func TestOffset(t *testing.T) {
	ranger := NewRanger(75)
	ranges := ranger.ranges(100, 20)
	assert.Equal(t, 2, len(ranges))
	assert.Equal(t, byteRange{from: 20, to: 74}, ranges[0])
	assert.Equal(t, byteRange{from: 75, to: 99}, ranges[1])
}

func TestOffsetAdvanced(t *testing.T) {
	ranger := NewRanger(10)
	ranges := ranger.ranges(100, 42)
	assert.Equal(t, 6, len(ranges))
	assert.Equal(t, byteRange{from: 42, to: 49}, ranges[0])
	assert.Equal(t, byteRange{from: 50, to: 59}, ranges[1])
	assert.Equal(t, byteRange{from: 90, to: 99}, ranges[5])
}

func TestHeader(t *testing.T) {
	// Needs to use the format `bytes=0-50`
	ranger := NewRanger(10)
	ranges := ranger.ranges(100, 0)
	assert.Equal(t, byteRange{from: 0, to: 9}, ranges[0])
	assert.Equal(t, "bytes=0-9", ranges[0].Header())
	assert.Equal(t, byteRange{from: 10, to: 19}, ranges[1])
	assert.Equal(t, "bytes=10-19", ranges[1].Header())
}
