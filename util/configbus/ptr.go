package configbus

// Ptr returns a pointer to v for StaticProvider scalar fields.
func Ptr[T any](v T) *T { return &v }

// PtrPtr returns a pointer to a pointer for StaticProvider fields whose Provider
// method returns *T. Nil outer means unset; outer set with nil inner means
// configured nil.
func PtrPtr[T any](v *T) **T { return &v }
