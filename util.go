package ranger

import "golang.org/x/exp/constraints"

func min[T constraints.Ordered](a T, b T) T {
	if a < b {
		return a
	}
	return b
}
