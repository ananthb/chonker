package chonker

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunk_RangeHeader(t *testing.T) {
	tests := []struct {
		name string
		c    Chunk
		want string
	}{
		{
			name: "zero",
			c:    Chunk{},
			want: "bytes=0-0",
		},
		{
			name: "valid range",
			c:    Chunk{Start: 10, Length: 10},
			want: "bytes=10-19",
		},
		{
			name: "valid range of 1 byte",
			c:    Chunk{Length: 1},
			want: "bytes=0-0",
		},
		{
			name: "zero length",
			c:    Chunk{Start: 10},
			want: "bytes=10-10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.c.RangeHeader())
		})
	}
}

func TestChunk_ContentRangeHeader(t *testing.T) {
	tests := []struct {
		name string
		c    Chunk
		size uint64
		want string
	}{
		{
			name: "zero",
			c:    Chunk{},
			want: "bytes */0",
		},
		{
			name: "no length and size",
			c:    Chunk{Start: 10},
			want: "bytes */0",
		},
		{
			name: "unsatisfied range",
			c:    Chunk{},
			size: 100,
			want: "bytes */100",
		},
		{
			name: "full",
			c:    Chunk{Start: 0, Length: 100},
			size: 100,
			want: "bytes 0-99/100",
		},
		{
			name: "partial",
			c:    Chunk{Start: 10, Length: 10},
			size: 100,
			want: "bytes 10-19/100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.c.ContentRangeHeader(tt.size))
		})
	}
}

var parseRangeTests = []struct {
	name    string
	s       string
	size    uint64
	want    []Chunk
	wantErr bool
}{
	{
		name:    "empty range",
		wantErr: true,
	},
	{
		name:    "invalid",
		s:       "keks=100500",
		size:    100,
		wantErr: true,
	},
	{
		name:    "invalid single value",
		s:       "bytes=200",
		size:    500,
		wantErr: true,
	},
	{
		name:    "invalid non-digit end",
		s:       "bytes=-f",
		size:    500,
		wantErr: true,
	},
	{
		name:    "invalid no start or end",
		s:       "bytes=-",
		size:    500,
		wantErr: true,
	},
	{
		name:    "invalid non-digit start",
		s:       "bytes=f-",
		size:    500,
		wantErr: true,
	},
	{
		name: "single",
		s:    "bytes=100-200",
		size: 200,
		want: []Chunk{
			{
				Start:  100,
				Length: 100,
			},
		},
	},
	{
		name: "multiple",
		s:    "bytes=100-199,300-399,500-599",
		size: 600,
		want: []Chunk{
			{
				Start:  100,
				Length: 100,
			},
			{
				Start:  300,
				Length: 100,
			},
			{
				Start:  500,
				Length: 100,
			},
		},
	},
	{
		name: "multiple with an empty range",
		s:    "bytes=100-199,300-399, ,500-599",
		size: 600,
		want: []Chunk{
			{
				Start:  100,
				Length: 100,
			},
			{
				Start:  300,
				Length: 100,
			},
			{
				Start:  500,
				Length: 100,
			},
		},
	},
	{
		name:    "no overlap",
		s:       "bytes=100-50",
		size:    200,
		wantErr: true,
	},
	{
		name:    "after end",
		s:       "bytes=200-250",
		size:    200,
		wantErr: true,
	},
	{
		name: "from offset till end",
		s:    "bytes=50-",
		size: 200,
		want: []Chunk{
			{
				Start:  50,
				Length: 150,
			},
		},
	},
	{
		name: "end greater than size",
		s:    "bytes=-250",
		size: 200,
		want: []Chunk{
			{
				Start:  0,
				Length: 200,
			},
		},
	},
}

func TestParseRange(t *testing.T) {
	for _, tt := range parseRangeTests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRange(tt.s, tt.size)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseRange() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func FuzzParseRange(f *testing.F) {
	for _, tt := range parseRangeTests {
		f.Add(tt.s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		r, err := ParseRange(s, 100)
		if err != nil {
			return
		}
		for _, rr := range r {
			if rr.Start == 0 && rr.Length == 0 {
				t.Fail()
			}
		}
	})
}

var parseContentRangeTests = []struct {
	name    string
	s       string
	want    *Chunk
	size    uint64
	wantErr bool
}{
	{
		name:    "blank",
		wantErr: true,
	},
	{
		name:    "invalid unit",
		s:       "keks=100500",
		wantErr: true,
	},
	{
		name:    "invalid single value",
		s:       "bytes 200",
		wantErr: true,
	},
	{
		name:    "invalid non-range",
		s:       "bytes abc/400",
		wantErr: true,
	},
	{
		name:    "invalid non-digit end",
		s:       "bytes -f/500",
		wantErr: true,
	},
	{
		name:    "invalid no start or end",
		s:       "bytes -/500",
		wantErr: true,
	},
	{
		name:    "invalid non-digit start",
		s:       "bytes f-/500",
		wantErr: true,
	},
	{
		name:    "invalid non-digit size",
		s:       "bytes 0-100/f",
		wantErr: true,
	},
	{
		name:    "invalid range larger than size",
		s:       "bytes 0-100/50",
		wantErr: true,
	},
	{
		name: "single",
		s:    "bytes 100-199/500",
		size: 500,
		want: &Chunk{
			Start:  100,
			Length: 100,
		},
	},
	{
		name:    "invalid start greater than end",
		s:       "bytes 100-50/500",
		wantErr: true,
	},
	{
		name:    "invalid no end",
		s:       "bytes 50-/500",
		wantErr: true,
	},
	{
		name:    "invalid no start",
		s:       "bytes -50/500",
		wantErr: true,
	},
	{
		name:    "unsatisfied range",
		s:       "bytes */500",
		size:    500,
		wantErr: true,
	},
}

func TestParseContentRange(t *testing.T) {
	for _, tt := range parseContentRangeTests {
		t.Run(tt.name, func(t *testing.T) {
			got, size, err := ParseContentRange(tt.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseContentRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) || size != tt.size {
				t.Errorf("ParseContentRange() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func FuzzParseContentRange(f *testing.F) {
	for _, tt := range parseContentRangeTests {
		f.Add(tt.s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		r, sz, err := ParseContentRange(s)
		if err != nil {
			return
		}
		if r.Start == 0 && r.Length == 0 && sz == 0 {
			t.Fail()
		}
	})
}

func TestChunks(t *testing.T) {
	tests := []struct {
		name      string
		chunkSize uint64
		offset    uint64
		length    uint64
		want      []Chunk
	}{
		{
			name:      "zero",
			chunkSize: 10,
			length:    0,
			want:      []Chunk{},
		},
		{
			name:      "ten ranges",
			chunkSize: 10,
			length:    87,
			want: []Chunk{
				{Start: 0, Length: 10},
				{Start: 10, Length: 10},
				{Start: 20, Length: 10},
				{Start: 30, Length: 10},
				{Start: 40, Length: 10},
				{Start: 50, Length: 10},
				{Start: 60, Length: 10},
				{Start: 70, Length: 10},
				{Start: 80, Length: 7},
			},
		},
		{
			name:      "overshoot",
			chunkSize: 75,
			offset:    0,
			length:    100,
			want: []Chunk{
				{Start: 0, Length: 75},
				{Start: 75, Length: 25},
			},
		},
		{
			name:      "offset",
			chunkSize: 10,
			offset:    12,
			length:    50,
			want: []Chunk{
				{Start: 12, Length: 8},
				{Start: 20, Length: 10},
				{Start: 30, Length: 10},
				{Start: 40, Length: 10},
			},
		},
		{
			name:      "huge chunks",
			chunkSize: 10000,
			offset:    57,
			length:    100,
			want: []Chunk{
				{Start: 57, Length: 43},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Chunks(tt.chunkSize, tt.offset, tt.length)
			assert.Equal(t, tt.want, got)
		})
	}
}
