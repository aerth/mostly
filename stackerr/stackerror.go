// Copyright (c) 2024 aerth
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
	"log"
	"strings"
)

// Wrap with stack trace from caller's caller (nil error returns nil)
func Wrap(err error, skips ...int) *StackError {
	if err == nil {
		return nil
	}
	st := GetFuncCallerInfo(skips...)
	return &StackError{error: err, St: st}
}

func containsErr(args []interface{}) bool {
	for _, arg := range args {
		if _, ok := arg.(error); ok {
			return true
		}
	}
	return false
}

var DEBUG = false

// Errorf with stack trace from caller. Use %w !
func Errorf(format string, args ...interface{}) *StackError {
	st := GetFuncCallerInfo()
	if len(args) != 0 && !strings.Contains(format, "%w") && containsErr(args) {
		if DEBUG {
			panic("stackerr.Errorf must contain %w")
		} else {
			log.Printf("stackerr.Errorf must contain %%w")
		}
	}
	return &StackError{error: fmt.Errorf(format, args...), St: st}
}

type StackError struct {
	error
	St FuncCallerInfo
}

var _ error = (*StackError)(nil)

func (s *StackError) Format(f fmt.State, c rune) {
	switch c {
	case 'v':
		if f.Flag('+') {
			fmt.Fprintf(f, "%+v", s.error)
			fmt.Fprintf(f, "\n\tfrom %s", s.St.String())
			//chl, ok := s.error.(*StackError)
			chld := new(StackError)
			if errors.As(s.error, &chld) { // recurse
				fmt.Fprintf(f, "\n\t\t%+v", chld)
			}
			return
		}
		fallthrough
	case 's':
		fmt.Fprint(f, s.Error())
	case 'q':
		fmt.Fprintf(f, "%q", s.Error())
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
