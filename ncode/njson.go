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

var DebugJsonRequests = true

// DecodeJsonReader does not close reader. set `DebugJsonRequests = true` for debug logging
func DecodeJsonReader[T any](rdr io.Reader) (T, error) {
	if DebugJsonRequests {
		return debugDecodeJsonReader[T](rdr)
	}
	var v T
	err := json.NewDecoder(rdr).Decode(&v)
	return v, err

}

// debug
func debugDecodeJsonReader[T any](rdr io.Reader) (T, error) {
	var v T
	buf, err := io.ReadAll(rdr)
	if err != nil {
		return v, err
	}
	var caller string
	for i := 1; i <= 6; i++ {
		_, file, num, ok := runtime.Caller(i)
		if !ok {
			break
		}
		fname := filepath.Base(file)
		if strings.HasPrefix(fname, "asm_") || strings.HasPrefix(fname, "xxxx") {
			break
		}
		caller += fmt.Sprintf("%s:%d ", fname, num)
	}
	log.Println("debugjson:", caller)
	os.Stderr.Write(buf)
	os.Stderr.Write([]byte{'\n'})
	return v, json.Unmarshal(buf, &v)
}
