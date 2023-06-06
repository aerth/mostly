// Copyright © 2023 aerth
// Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the “Software”), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
// The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
// THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package ncode

import "errors"

var ErrSkip = errors.New("skip error")

// Twist strings into things, O(n)
func Twist[T ~string](a []string) []T {
	return TwistAny(a, func(k string) T {
		return T(k)
	})
}

// TwistBack things into strings, O(n)
func TwistBack[T ~string](a []T) []string {
	return TwistAny(a, func(k T) string {
		return string(k)
	})
}

// TwistParse strings into any things (with error handling)
// skip elements (but continue indexes) by returning ErrSkip in fn function
// TODO: skip without incrementing index
func TwistParse[T any](a []string, fn func(s string) (T, error)) ([]T, error) {
	return TwistAnyWithError(a, fn)
}

// TwistAnyWithError in case theres a parsing issue.
// skip elements (but continue indexes) by returning ErrSkip in fn function
// TODO: skip without incrementing index
func TwistAnyWithError[T any, K any](a []K, fn func(s K) (T, error)) ([]T, error) {
	k := make([]T, len(a))
	for i := range a {
		x, err := fn(a[i])
		if err == ErrSkip {
			continue
		}
		if err != nil {
			return k, err
		}
		k[i] = x
	}
	return k, nil
}

// TwistFormat array into a string array
func TwistFormat[T any](a []T, fn func(v T) string) []string {
	return TwistAny(a, fn)
}

// TwistAny array into another kind of array
func TwistAny[T any, K any](a []T, fn func(v T) K) []K {
	k := make([]K, len(a))
	for i := range a {
		k[i] = fn(a[i])
	}
	return k
}
