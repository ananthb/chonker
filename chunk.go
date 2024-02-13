// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package chonker

import (
	"errors"
	"fmt"
	"net/textproto"
	"strconv"
	"strings"
)

var (
	// ErrRangeNoOverlap is returned by ParseRange if first-byte-pos of
	// all of the byte-range-spec values is greater than the content size.
	ErrRangeNoOverlap = errors.New("chonker: ranges failed to overlap")

	// ErrInvalidRange is returned by ParseRange on invalid input.
	ErrInvalidRange = errors.New("chonker: invalid range")

	// ErrUnsatisfiedRange is returned by ParseContentRange if the range is not satisfied.
	ErrUnsatisfiedRange = errors.New("chonker: unsatisfied range")
)

// Chunk represents a byte range.
type Chunk struct {
	Start  uint64
	Length uint64
}

// RangeHeader returns a RangeHeader header value.
// A zero length is treated as a single byte range.
// For more information on the Range header, see the MDN article on the
// [Range header].
//
// [Range header]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Range
func (c Chunk) RangeHeader() string {
	end := c.Start + c.Length
	if c.Length != 0 {
		end--
	}
	return fmt.Sprintf("bytes=%d-%d", c.Start, end)
}

// ContentRangeHeader returns a Content-Range header value.
// Size is the total size of the content.
// Calling this method on a zero-value Chunk will return an unsatisfied range.
// For more information on the Content-Range header, see the MDN article on
// the [Content-Range header].
//
// [Content-Range header]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Range
func (c Chunk) ContentRangeHeader(size uint64) string {
	const unit = "bytes "
	// We only have a size, so return an unsatisfied range.
	if c.Start == 0 && c.Length == 0 && size != 0 {
		return fmt.Sprintf("%s*/%d", unit, size)
	}
	end := c.Start + c.Length
	if c.Length != 0 {
		end--
	}
	if c.Length == 0 || c.Length > size {
		return fmt.Sprintf("%s*/%d", unit, size)
	}
	return fmt.Sprintf("%s%d-%d/%d", unit, c.Start, end, size)
}

// ParseRange parses a Range header string as per [RFC 7233].
// ErrNoOverlap is returned if none of the ranges fit inside content size.
// This function is a copy of the [parseRange] function from the Go standard library
// net/http/fs.go with minor modifications.
//
// [RFC 7233]: https://tools.ietf.org/html/rfc7233#section-3.1
// [parseRange]: https://github.com/golang/go/blob/b4fa5b163df118b35a836bbe5706ac268b4cc14b/src/net/http/fs.go#L956
func ParseRange(s string, size uint64) ([]Chunk, error) {
	const b = "bytes="
	if !strings.HasPrefix(s, b) {
		return nil, ErrInvalidRange
	}
	s = s[len(b):]
	chunks := make([]Chunk, 0)
	noOverlap := false
	for _, ra := range strings.Split(s, ",") {
		ra = textproto.TrimString(ra)
		if ra == "" {
			continue
		}
		i := strings.Index(ra, "-")
		if i < 0 {
			return nil, ErrInvalidRange
		}
		start, end := textproto.TrimString(ra[:i]), textproto.TrimString(ra[i+1:])
		c := Chunk{}
		if start == "" {
			// If no start is specified, end specifies the
			// range start relative to the end of the file,
			// and we are dealing with <suffix-length>
			// which has to be a non-negative integer as per
			// RFC 7233 Section 2.1 "Byte-Ranges".
			if end == "" || end[0] == '-' {
				return nil, ErrInvalidRange
			}
			i, err := strconv.ParseInt(end, 10, 64)
			if i < 0 || err != nil {
				return nil, ErrInvalidRange
			}
			if sz := int64(size); i > sz {
				i = sz
			}
			c.Start = size - uint64(i)
			c.Length = size - c.Start
		} else {
			i, err := strconv.ParseInt(start, 10, 64)
			if err != nil || i < 0 {
				return nil, ErrInvalidRange
			}
			if i >= int64(size) {
				// If the range begins after the size of the content,
				// then it does not overlap.
				noOverlap = true
				continue
			}
			c.Start = uint64(i)
			if end == "" {
				// If no end is specified, range extends to end of the file.
				c.Length = size - c.Start
			} else {
				i, err := strconv.ParseInt(end, 10, 64)
				if err != nil || int64(c.Start) > i {
					return nil, ErrInvalidRange
				}
				if i >= int64(size) {
					i = int64(size) - 1
				}
				c.Length = uint64(i) - c.Start + 1
			}
		}
		chunks = append(chunks, c)
	}
	if noOverlap && len(chunks) == 0 {
		// The specified ranges did not overlap with the content.
		return nil, ErrRangeNoOverlap
	}
	return chunks, nil
}

// ParseContentRange parses a Content-Range header string as per [RFC 7233].
// It returns the chunk describing the returned content range, and the size of the content.
// ErrUnsatisfiedRange is returned if the range is not satisfied.
//
// [RFC 7233]: https://tools.ietf.org/html/rfc7233#section-4.2
func ParseContentRange(s string) (*Chunk, uint64, error) {
	const bs = "bytes "
	if !strings.HasPrefix(s, bs) {
		return nil, 0, ErrInvalidRange
	}
	s = s[len(bs):]
	b, a, ok := strings.Cut(s, "/")
	if !ok {
		return nil, 0, ErrInvalidRange
	}
	size, err := strconv.ParseInt(a, 10, 64)
	if err != nil {
		return nil, 0, err
	}
	if b == "*" {
		return nil, uint64(size), ErrUnsatisfiedRange
	}
	b, a, ok = strings.Cut(b, "-")
	if !ok {
		return nil, 0, ErrInvalidRange
	}
	start, err := strconv.ParseInt(b, 10, 64)
	if err != nil {
		return nil, 0, err
	}
	end, err := strconv.ParseInt(a, 10, 64)
	if err != nil {
		return nil, 0, err
	}
	if start > end || end > size {
		return nil, 0, ErrInvalidRange
	}
	c := &Chunk{
		Start:  uint64(start),
		Length: uint64(end - start + 1),
	}
	return c, uint64(size), nil
}

// index returns the index of the chunk containing the given offset.
func index(chunkSize, offset uint64) uint64 {
	return offset / chunkSize
}

// Chunks divides the range [offset, size) into chunks of size chunkSize.
func Chunks(chunkSize, offset, size uint64) []Chunk {
	ranges := make([]Chunk, 0)
	for i := index(chunkSize, offset) * chunkSize; i < size; i += chunkSize {
		c := Chunk{
			Start:  i,
			Length: min(chunkSize, size-i),
		}
		// If the first chunk is offset, nudge it to the right.
		if len(ranges) == 0 {
			nudge := offset % chunkSize
			c.Start += nudge
			c.Length -= nudge
		}
		ranges = append(ranges, c)
	}
	return ranges
}

func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
