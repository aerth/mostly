package filters

// FilterInPlace modifies the slice in place. See Warning.
//
// Warning: All slices that share the same backing array will be modified and need to be replaced by the return value.
func FilterInPlace[S ~[]T, T any](a S, keepfn func(a T) bool) S {
	if len(a) == 0 {
		return a
	}
	good := 0
	l := len(a)
	for i := 0; i < l; i++ {
		if keepfn(a[i]) {
			if i != good {
				a[good] = a[i] // will be equal if all good so far
			}
			good++
		}
	}
	return a[:good] // will be same slice if all pass keepfn
}

// FilterCopy returns a new slice with only the items that pass the filter
// The original slice is not modified.
//
// For less memory usage, use FilterInPlace
func FilterCopy[S ~[]T, T any](a S, keepfn func(a T) bool) (out S) {
	if len(a) == 0 {
		return a
	}
	l := len(a)
	for i := 0; i < l; i++ {
		if keepfn(a[i]) {
			out = append(out, a[i])
		}
	}
	return
}
