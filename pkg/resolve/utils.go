package resolve

func Any[T any](s []T, f func(T) bool) bool {
	for _, t := range s {
		if f(t) {
			return true
		}
	}
	return false
}
