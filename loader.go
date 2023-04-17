package ranger

// Loader implements a Load method that provides data as byte slice for a
// given byte range chunk.
//
// `Load` should be safe to call from multiple goroutines.
//
// If err is nil, the returned byte slice must always have exactly as many bytes as was
// asked for, i.e. `len([]byte)` returned must always be equal to `br.Ranges()`.
type Loader interface {
	Load(br ByteRange) ([]byte, error)
}

// LoaderFunc converts a Load function into a Loader type.
type LoaderFunc func(br ByteRange) ([]byte, error)

func (l LoaderFunc) Load(br ByteRange) ([]byte, error) {
	return l(br)
}
