// Copyright (c) 2024-2026 aerth
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package stackerr

import (
	"errors"
	"fmt"
	"strings"
)

// Wrap with stack trace from caller's caller (nil error returns nil)
//
// See [WrapN] to use additional caller skip
func Wrap(err error) error {
	if err == nil {
		return nil
	}
	st := GetFuncCallerInfo()
	return &StackError{error: fmt.Errorf("%w", err), St: st}
}

// WrapN with stack trace from caller's caller (nil error returns nil)
func WrapN(err error, skip int) *StackError {
	if err == nil {
		return nil
	}
	st := GetFuncCallerInfo(skip)
	return &StackError{error: fmt.Errorf("%w", err), St: st}
}

// Errorf with stack trace from caller. Always use %w to wrap errors
//
// example:
//
//	return stackerr.Errorf("%w", err)
//
// Then elsewhere,
//
// > log.Printf("Error: %+v", err)
func Errorf(format string, args ...interface{}) error {
	st := GetFuncCallerInfo()
	return &StackError{error: fmt.Errorf(format, args...), St: st}
}

// StackError wraps an error and holds the stack trace St ([FuncCallerInfo])
//
// Create with [Errorf] or [Wrap] (`stackerr.Errorf("%w", err)`)
// Print with `%+v` verb (or call err.Stack())
// Use err.Unwrap() (or errors.Unwrap(err)) to retrieve underlying error
//
// Example:
//
//	err := foo()
//	if err != nil {
//	  return fmt.Errorf("%w", err)
//	}
//
//	in main, `log.Fatalf("%+v", err)`
type StackError struct {
	error
	St FuncCallerInfo `json:"caller"`
}

// M do not use
func M(err error, st FuncCallerInfo) *StackError {
	return &StackError{error: err, St: st}
}

var _ error = (*StackError)(nil)

// MarshalText for getting slog package to print %+z mode (no newlines)
func (s *StackError) MarshalText() ([]byte, error) {
	x := fmt.Sprintf("%+z", s)
	return []byte(x), nil
}

// Format implements [fmt.Formatter]
//
// %+v for nice stack trace, %+z for single-line version suitable for slog package
func (s *StackError) Format(f fmt.State, c rune) {
	switch c {
	case 'z':
		if f.Flag('+') {
			m := fmt.Sprintf("%+v", s)
			_, _ = strings.NewReplacer("\n", " ", "\t", "  ").WriteString(f, m)
			return
		}
		fallthrough
	case 'y': // internal for recursing without first line
		if f.Flag('+') {
			_, _ = fmt.Fprintf(f, "from %s", s.St.String())
			c, ok := errors.AsType[*StackError](s.error)
			if ok {
				_, _ = fmt.Fprintf(f, "\n\t%+y", c) // recurse unwrapped
			}
			return
		}
		fallthrough
	case 'v':
		if f.Flag('+') {
			_, _ = fmt.Fprintf(f, "%+v", s.error)
			_, _ = fmt.Fprintf(f, "\n\t%+y", s) // recurse
			return
		}
		fallthrough
	case 's':
		_, _ = fmt.Fprint(f, s.Error())
	case 'q':
		_, _ = fmt.Fprintf(f, "%q", s.Error())
	default:
		panic(fmt.Sprintf("verb %%%c is not supported for stackerr.Error: %v", c, s.error))
	}
}

func (s *StackError) Underlying() error {
	return s.error
}

func (s *StackError) Unwrap() error {
	return s.error
}

func (s *StackError) Stack() FuncCallerInfo {
	return s.St
}
