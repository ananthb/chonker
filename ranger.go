package ranger

// Chunk represents a chunk of data that can be loaded from a source.
type Chunk struct {
	Loader    Loader
	ByteRange ByteRange
}

// Load loads the chunk from the source.
func (c *Chunk) Load() ([]byte, error) {
	return c.Loader.Load(c.ByteRange)
}

// Ranger can split a file into chunks of a given size.
type Ranger struct {
	chunkSize int64
}

func NewRanger(chunkSize int64) Ranger {
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
