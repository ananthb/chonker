package ranger

type Chunk struct {
	Loader    Loader
	ByteRange ByteRange
}

func (c *Chunk) Load() ([]byte, error) {
	return c.Loader.Load(c.ByteRange)
}

type Ranger struct {
	chunkSize int64
}

func NewRanger(chunkSize int64) Ranger {
	return Ranger{chunkSize: chunkSize}
}

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

func (r Ranger) Index(i int64) int {
	// we want a math.floor on the index / chunk size - this gives us the index of the chunk
	return int(i / r.chunkSize)
}
