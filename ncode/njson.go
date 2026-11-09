// Copyright © 2023 aerth
// Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the “Software”), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
// The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
// THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package ncode

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var DebugJSONRequests = false

// DecodeJSONReader does not close reader. set `DebugJSONRequests = true` for debug logging
//
// See [DecodeJSONReadCloser]
func DecodeJSONReader[T any](rdr io.Reader) (T, error) {
	if DebugJSONRequests {
		return debugDecodeJSONReader[T](rdr)
	}
	var v T
	err := json.NewDecoder(rdr).Decode(&v)
	return v, err
}

func DecodeJSONReadCloser[T any](r io.ReadCloser) (T, error) {
	defer func() { _ = r.Close() }()
	return DecodeJSONReader[T](r)
}

// debug
func debugDecodeJSONReader[T any](rdr io.Reader) (T, error) {
	var v T
	buf, err := io.ReadAll(rdr)
	if err != nil {
		return v, err
	}
	var caller strings.Builder
	for i := 1; i <= 6; i++ {
		_, file, num, ok := runtime.Caller(i)
		if !ok {
			break
		}
		fname := filepath.Base(file)
		if strings.HasPrefix(fname, "asm_") || strings.HasPrefix(fname, "xxxx") {
			break
		}
		fmt.Fprintf(&caller, "%s:%d ", fname, num)
	}
	log.Println("debugjson:", caller.String())
	_, _ = os.Stderr.Write(buf)
	_, _ = os.Stderr.Write([]byte{'\n'})
	return v, json.Unmarshal(buf, &v)
}
