// Package filters for filtering, both in place and copy
package filters

// FilterInPlace ...
//
// Deprecated: As of mostly v0.4.5, this function simply calls [InPlace] and will be removed in future versions.
func FilterInPlace[S ~[]T, T any](a S, keepfn func(a T) bool) S {
	return InPlace(a, keepfn)
}

// FilterCopy ...
//
// Deprecated: As of mostly v0.4.5, this function simply calls [Copy] and will be removed in future versions.
func FilterCopy[S ~[]T, T any](a S, keepfn func(a T) bool) S {
	return Copy(a, keepfn)
}

// InPlace modifies the slice in place. See Warning.
//
// Warning: All slices that share the same backing array will be modified and need to be replaced by the return value.
func InPlace[S ~[]T, T any](a S, keepfn func(a T) bool) S {
	if len(a) == 0 {
		return a
	}
	good := 0
	l := len(a)
	for i := range l {
		if keepfn(a[i]) {
			if i != good {
				a[good] = a[i] // will be equal if all good so far
			}
			good++
		}
	}
	return a[:good] // will be same slice if all pass keepfn
}

// Copy returns a new slice with only the items that pass the filter
// The original slice is not modified.
//
// For less memory usage, use FilterInPlace
func Copy[S ~[]T, T any](a S, keepfn func(a T) bool) (out S) {
	if len(a) == 0 {
		return a
	}
	l := len(a)
	for i := range l {
		if keepfn(a[i]) {
			out = append(out, a[i])
		}
	}
	return
}

// InvertBoolFunc to invert functions to true=keep that were designed for true=delete [slices.DeleteFunc]
func InvertBoolFunc[T any](fn func(a T) bool) func(a T) bool {
	return func(v T) bool {
		return !fn(v)
	}
}
