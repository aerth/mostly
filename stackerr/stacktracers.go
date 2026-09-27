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
	"fmt"
	"path/filepath"
	"runtime"
	"time"
)

type FuncCallerInfo struct {
	Name string    `json:"name,omitempty"`
	File string    `json:"file,omitempty"`
	Line int       `json:"line,omitempty"`
	Time time.Time `json:"time,omitempty"`
}

func (fci FuncCallerInfo) String() string {
	return fmt.Sprintf("%s (%s:%d)", fci.Name, filepath.Base(fci.File), fci.Line)
}

func GetFuncCallerInfo(skips ...int) FuncCallerInfo {
	if len(skips) > 1 {
		panic("GetFuncCallerInfo: too many skip arguments")
	}
	skip := 2 // skip GetFuncCallerInfo and it's caller
	if len(skips) > 0 {
		skip += skips[0] // and additional
	}
	pc, tfile, tline, ok := runtime.Caller(skip) // skip caller of this function
	var details *runtime.Func
	if ok {
		details = runtime.FuncForPC(pc)
	}
	name := "unknown"
	if details != nil {
		// Name() returns something like: github.com/aerth/mostly/stackerr_test.TestError2a
		name = filepath.Base(details.Name())
	}
	return FuncCallerInfo{
		Name: name,
		File: tfile,
		Line: tline,
		Time: time.Now(),
	}
}
