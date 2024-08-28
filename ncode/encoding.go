// Copyright © 2023 aerth
// Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the “Software”), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
// The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
// THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package ncode

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/aerth/mostly/ncode/constraints"
)

var Binary = binary.LittleEndian
var Json = toJson

// eg: ncode.Json = ncode.JsonIndent
func JsonIndent(a any) []byte {
	b, _ := json.MarshalIndent(a, "", " ")
	return b
}
func JsonString(a any) string {
	return string(toJson(a))
}
func toJson(a any) []byte {
	b, _ := json.Marshal(a)
	return b
}

// ErrZeroLength 404 not found
var ErrZeroLength = fmt.Errorf("cannot decode zero length")

func DecodeJson[T any](b []byte) (T, error) {
	var v T
	if len(b) == 0 {
		return v, ErrZeroLength
	}
	err := json.Unmarshal(b, &v)
	return v, err
}

// N2B Number to []byte
func N2B[E constraints.Unsigned](n E) []byte {
	var buf = make([]byte, 8)
	Binary.PutUint64(buf, uint64(n))
	return buf
}

// B2N []byte to number
func B2N(b []byte) uint64 {
	return Binary.Uint64(b)
}
