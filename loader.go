package ranger

import (
	"os"
	"path"

	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/sync/singleflight"
)

// Loader implements a Load method that provides data as byte slice for a
// given byte range chunk.
//
// `Load` should be safe to call from multiple goroutines.
//
// If err is nil, the returned byte slice must always have exactly as many bytes as was
// asked for, i.e. `len([]byte)` returned must always be equal to `br.Length()`.
type Loader interface {
	Load(br ByteRange) ([]byte, error)
}

// LoaderFunc converts a Load function into a Loader type.
type LoaderFunc func(br ByteRange) ([]byte, error)

func (l LoaderFunc) Load(br ByteRange) ([]byte, error) {
	return l(br)
}

// WrapLoaderWithSingleFlight wraps a Loader to ensure that only one call at a time
// for a given byte range is made to the wrapped loader. This effectively serializes
// calls to the wrapped loader for a given byte range, allowing lock-free and mutex-free
// operations. Load calls for different byte ranges can still happen in parallel.
func WrapLoaderWithSingleFlight(loader Loader) Loader {
	group := new(singleflight.Group)
	return LoaderFunc(func(br ByteRange) ([]byte, error) {
		data, err, _ := group.Do(br.RangeHeader(), func() (interface{}, error) {
			data, err := loader.Load(br)
			return data, err
		})
		return data.([]byte), err
	})
}

// WrapLoaderWithLRUCache wraps a loader to cache the results returned by the
// inner loader in an LRU cache with the given slot count. For best results, wrap the returned
// Loader with WrapLoaderWithSingleFlight to make sure multiple calls are not
// made while the cache is being filled.
//
// If the given slots count is negative, zero is used.
func WrapLoaderWithLRUCache(loader Loader, slots int) Loader {
	cache, _ := lru.New[ByteRange, []byte](max(slots, 0))
	return LoaderFunc(func(br ByteRange) ([]byte, error) {
		if data, found := cache.Get(br); found {
			return data, nil
		}

		data, err := loader.Load(br)
		if err == nil {
			cache.Add(br, data)
		}

		return data, nil
	})
}

// WrapLoaderWithFileCache wraps a loader to cache the data returned in the given directory.
//
// Each byte range is cached in a separate file named after the byte range header.
//
// The cache is not invalidated or cleared in any way, so the caller is responsible for
// cleaning up the cache directory and invalidating it on remote changes.
func WrapLoaderWithFileCache(loader Loader, cacheDir string) Loader {
	return LoaderFunc(func(br ByteRange) ([]byte, error) {
		filename := path.Join(cacheDir, br.RangeHeader())
		data, err := os.ReadFile(filename)
		if err == nil {
			return data, nil
		}
		data, err = loader.Load(br)
		if err == nil {
			_ = os.WriteFile(filename, data, 0644)
		}
		return data, err
	})
}

// WrapLoaderWithLoadSheddingFileCache behavior is the same as WrapLoaderWithFileCache,
// except that it will shed load by not using the cache if maxLoad operations are already
// waiting to be preformed. This allows bypassing a cache on a slow or busy filesystem.
func WrapLoaderWithLoadSheddingFileCache(loader Loader, cacheDir string, maxLoad int) Loader {
	readChan := make(chan struct{}, max(maxLoad, 1))
	writeChan := make(chan struct{}, max(maxLoad, 1))

	return LoaderFunc(func(br ByteRange) ([]byte, error) {
		filename := path.Join(cacheDir, br.RangeHeader())

		select {
		case readChan <- struct{}{}:
			data, err := os.ReadFile(filename)
			<-readChan
			if err == nil {
				return data, nil
			}
		default:
		}

		data, err := loader.Load(br)

		select {
		case writeChan <- struct{}{}:
			_ = os.WriteFile(filename, data, 0644)
			<-writeChan
		default:
		}

		return data, err
	})
}
