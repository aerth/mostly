// Package vector256 represents a 256-dimensional float32 vector for pgvector
// part of aerth/mostly project
package vector256

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
)

// Vector256 represents a 256-dimensional float32 vector for pgvector and may be zero-length or a slice of 256 float32
type Vector256 []float32

// SearchEmbeddings helper embedded type (for 256 specific)
type SearchEmbeddings struct {
	Embeddings Vector256 `gorm:"type:vector(256);default:null" json:"-"`
}

var VarLen = 256

// Value converts the Go float32 slice into the '[0.1,0.2,...]' string format Postgres expects
func (v Vector256) Value() (driver.Value, error) {
	if v == nil {
		return nil, nil
	}
	if len(v) == 0 {
		return nil, nil
	}
	if len(v) != VarLen {
		return nil, fmt.Errorf("invalid vector length: got %d, expected %d", len(v), VarLen)
	}

	// TODO: optimize
	strVals := make([]string, len(v))
	for i, val := range v {
		strVals[i] = strconv.FormatFloat(float64(val), 'f', -1, 32)
	}
	return "[" + strings.Join(strVals, ",") + "]", nil
}

// Scan converts the Postgres database value back into a Go float32 slice
func (v *Vector256) Scan(value any) error {
	if value == nil {
		*v = nil
		return nil
	}
	var str string
	switch data := value.(type) {
	case string:
		str = data
	case []byte:
		str = string(data)
	default:
		return fmt.Errorf("cannot scan type %T into Vector256", value)
	}
	str = strings.Trim(str, "[]")
	if str == "" {
		*v = Vector256{}
		return nil
	}
	tokens := strings.Split(str, ",")
	if len(tokens) != VarLen {
		return fmt.Errorf("invalid database vector dimension: got %d, expected %d", len(tokens), VarLen)
	}

	res := make([]float32, len(tokens))
	for i, t := range tokens {
		val, err := strconv.ParseFloat(strings.TrimSpace(t), 32)
		if err != nil {
			return fmt.Errorf("failed to parse vector element at index %d: %w", i, err)
		}
		res[i] = float32(val)
	}

	*v = res
	return nil
}
