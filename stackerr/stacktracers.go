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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
)

type FuncCallerInfo struct {
	funcname string
	filetag  string
}

func (fci FuncCallerInfo) String() string {
	return fmt.Sprintf("%s (%s)", fci.funcname, fci.filetag)
}

func GetFuncCallerInfo(skips ...int) FuncCallerInfo {
	if len(skips) > 1 {
		panic("GetFuncCallerInfo: too many skip arguments")
	}
	skip := 2
	if len(skips) > 0 {
		skip += skips[0]
	}
	pc, tfile, tline, ok := runtime.Caller(skip) // skip caller of this function
	details := runtime.FuncForPC(pc)
	if ok && details != nil {
		tfile = Cleanmodulepath(tfile)
		//fmt.Printf("called from %s (%s:%d)\n", filepath.Base(details.Name()), tfile, tline)
	}
	return FuncCallerInfo{
		funcname: filepath.Base(details.Name()),
		filetag:  fmt.Sprintf("%s:%d", tfile, tline),
	}
}

// var mainmoduleprefix string
var mainmodulecmdprefix string
var mainpwd string

func init() {
	buildinfo, ok := debug.ReadBuildInfo()
	if !ok {
		log.Printf("Failed to read build info")
		return
	}
	// fmt.Println("mainmoduleprefix:", buildinfo.Path)
	// mainmoduleprefix = buildinfo.Path
	dir, err := os.Getwd()
	if err != nil {
		log.Printf("Failed to get working directory")
		return
	}
	mainpwd = dir
	mainmodulecmdprefix = buildinfo.Main.Path
}
func Cleanmodulepath(p string) string {
	p1 := p
	p = strings.TrimPrefix(p, mainmodulecmdprefix)
	if mainpwd != "/" {
		p = strings.TrimPrefix(p, mainpwd)
		if strings.HasSuffix(mainpwd, "/") && !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
	}
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return p1
	}
	return p
}
