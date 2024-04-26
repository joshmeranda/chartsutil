package rebase

func ToPtr[T any](t T) *T {
	return &t
}
