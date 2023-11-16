package ranger

const (
	defaultChunkSize = 1024 * 1024 // 1MB
)

// Ranger can split a file into chunks of a given size.
type Ranger struct {
	chunkSize int64
}

// NewRanger creates a new Ranger with the given chunk size.
// If the chunk size is <= 0, the default chunk size is used.
func NewRanger(chunkSize int64) Ranger {
	if chunkSize <= 0 {
		chunkSize = defaultChunkSize
	}
	return Ranger{chunkSize: chunkSize}
}

// Ranges creates a list of byte ranges with the given chunk size.
func (r Ranger) Ranges(length int64) []ByteRange {
	ranges := make([]ByteRange, 0)
	for i := int64(0); i < length; i += r.chunkSize {
		br := ByteRange{
			From: i,
			To:   min(i+r.chunkSize-1, length-1),
		}
		ranges = append(ranges, br)
	}
	return ranges
}

// Index returns the index of the chunk that contains the given offset.
func (r Ranger) Index(i int64) int {
	// we want a math.floor on the index / chunk size - this gives us the index of the chunk
	return int(i / r.chunkSize)
}

type SizedRanger struct {
	length int64
	ranger Ranger
}

func (r SizedRanger) Length() int64 {
	return r.length
}

func (r SizedRanger) RangeContaining(offset int64) (br ByteRange) {
	index := r.ranger.Index(offset)
	ranges := r.ranger.Ranges(r.length)
	if index < len(ranges) {
		br = ranges[index]
	}
	return
}

func NewSizedRanger(length int64, ranger Ranger) SizedRanger {
	return SizedRanger{
		length: length,
		ranger: ranger,
	}
}
