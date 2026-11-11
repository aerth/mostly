// Copyright © 2023 aerth
// Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the “Software”), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
// The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
// THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package ncode

import (
	"errors"
)

var ErrSkip = errors.New("skip error")

// Twist strings into things, O(n)
func Twist[T ~string | ~[]byte](a []string) []T {
	return TwistAny(a, func(k string) T {
		return T(k)
	})
}

// TwistBack things into strings, O(n)
func TwistBack[T ~string | ~[]byte](a []T) []string {
	return TwistAny(a, func(k T) string {
		return string(k)
	})
}

// TwistParse2 strings into any things (with error handling)
// skip elements (but continue indexes) by returning [ErrSkip] in fn function
//
// Example:
//
//	ncode.TwistParse2[url.URL]([]string{"google.com", "myspace.com"}, url.Parse2)
//
// TODO: skip without incrementing index
func TwistParse2[T any, A any, B any](a []A, b B, fn2 func(a A, b B) (T, error)) ([]T, error) {
	fn := func(a A) (T, error) {
		return fn2(a, b)
	}

	return TwistAnyWithError(a, fn)
}

func TwistParse2Ptr[T any, A any, B any](a []A, b B, fn2 func(a A, b B) (*T, error)) ([]*T, error) {
	fn := func(a A) (*T, error) {
		return fn2(a, b)
	}
	return TwistAnyWithError(a, fn)
}

// TwistParse strings into any things (with error handling)
// skip elements (but continue indexes) by returning [ErrSkip] in fn function
//
// Example:
//
//	ncode.TwistParse[url.URL]([]string{"google.com", "myspace.com"}, url.Parse)
//
// TODO: skip without incrementing index
func TwistParse[T any](a []string, fn func(s string) (T, error)) ([]T, error) {
	return TwistAnyWithError(a, fn)
}

func TwistParsePtr[T any](a []string, fn func(s string) (*T, error)) ([]*T, error) {
	return TwistAnyWithError(a, fn)
}

type NCode[T any, K any] struct {
	NonFatalErrors bool
	Fn             func(s K) (T, error)
}

func Twister[T any, V any](fn func(s V) (T, error)) NCode[T, V] {
	return NCode[T, V]{Fn: fn}
}

func (n NCode[T, K]) Twist(a []K) ([]T, error) {
	if n.NonFatalErrors {
		return TwistAnySkipError(a, n.Fn)
	}
	return TwistAnyWithError(a, n.Fn)
}

// TwistAnySkipError in case theres a parsing issue.
// Errored are skipped and items will be zero-value in returned slice.
//
// Errors are collected
func TwistAnySkipError[T any, K any](a []K, fn func(s K) (T, error)) ([]T, error) {
	var errs error
	k := make([]T, len(a))
	for i := range a {
		x, err := fn(a[i])
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		k[i] = x
	}
	return k, errs
}

// TwistAnyWithError in case theres a parsing issue.
// skip elements (but continue indexes) by returning ErrSkip in fn function
//
// Skipped items will be zero-value in returned slice.
func TwistAnyWithError[T any, K any](a []K, fn func(s K) (T, error)) ([]T, error) {
	k := make([]T, len(a))
	for i := range a {
		x, err := fn(a[i])
		if errors.Is(err, ErrSkip) {
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

var (
	S2B = VarList[string, []string]
	ANY = VarList[any, []any]
)

func VarList[T any, A ~[]T](a ...T) A {
	return a
}
