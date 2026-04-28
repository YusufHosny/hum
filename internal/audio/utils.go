package audio

import "cmp"

func clamp[T cmp.Ordered](value T, low T, high T) T {
	return max(min(value, high), low)
}
