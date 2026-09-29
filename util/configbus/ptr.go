package configbus

// Ptr returns a pointer to v for StaticProvider scalar and struct fields.
func Ptr[T any](v T) *T { return &v }
