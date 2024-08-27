// unixtimestamp package provides a time.Time wrapper that marshals to/from Unix timestamp (integer, seconds)
package unixtimestamp

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"time"
)

var Errorf = fmt.Errorf

// New existing time.Time
func New(t time.Time) *UnixTimestamp {
	return &UnixTimestamp{t}
}

// Now returns a new UnixTimestamp for the current time
func Now() *UnixTimestamp {
	return New(time.Now())
}

// Endian is the default byte order for UnixTimestamp for MarshalBinary interface
var Endian = binary.LittleEndian

// UnixTimestamp is a time.Time that marshals to/from Unix timestamp (seconds)
// Use pointer (*UnixTimestamp) and `omitempty` in structs for JSON marshaling
// Use Wrap(t) or Now() to create a UnixTimestamp
type UnixTimestamp struct {
	time.Time
}

func (ut UnixTimestamp) MarshalJSON() ([]byte, error) {
	if ut.Time.After(zerotime) {
		return []byte(strconv.Itoa(int(FuncFrom(ut.Time)))), nil
	}
	return []byte("0"), nil
}

func (ut *UnixTimestamp) UnmarshalJSON(dat []byte) error {
	unix, err := strconv.Atoi(string(dat))
	if err != nil {
		return err
	}
	if unix == 0 {
		ut.Time = time.Time{}
		return nil
	}
	ut.Time = FuncTo(int64(unix))
	return nil
}

var FuncTo = TSeconds   // consider TMilli
var FuncFrom = FSeconds // Change FuncTo also. consider FMilli.

func FSeconds(t time.Time) int64 {
	return t.Unix()
}

func FMilli(t time.Time) int64 {
	return t.UnixMilli()
}

func FMicro(t time.Time) int64 {
	return t.UnixMicro()
}

func TSeconds(i int64) time.Time {
	return time.Unix(i, 0)
}

func TMilli(i int64) time.Time {
	return time.UnixMilli(i)
}

func TMicro(i int64) time.Time {
	return time.UnixMicro(i)
}

// MarshalBinary uses Endian var, set Endian to binary.BigEndian if needed
func (ut UnixTimestamp) MarshalBinary() ([]byte, error) {
	var buf [8]byte
	if ut.Time.After(zerotime) {
		Endian.PutUint64(buf[:], uint64(FuncFrom(ut.Time)))
	}
	return buf[:], nil
}

// UnmarshalBinary uses Endian var, set Endian to binary.BigEndian if needed
func (ut *UnixTimestamp) UnmarshalBinary(dat []byte) error {
	ut.Time = FuncTo(int64(Endian.Uint64(dat)))
	if !ut.Time.After(zerotime) {
		ut.Time = time.Time{}
	}
	return nil
}

func (u *UnixTimestamp) Scan(v interface{}) error {
	if v == nil {
		return nil
	}

	switch x := v.(type) {
	case time.Time:
		u.Time = x
		return nil
	case int:
		u.Time = FuncTo(int64(x))
	case int32:
		u.Time = FuncTo(int64(x))
	case int64:
		u.Time = FuncTo(int64(x))
	case string:
		t, err := time.Parse(time.RFC3339, x)
		if err != nil {
			return err
		}
		u.Time = t
		return nil
	default:
		return Errorf("unsupported type: %T", v)
	}

	if !NoCheckTimeScan {
		// check for valid time
		if u.Time.Before(zerotime) {
			return Errorf("time too early: %v", u.Time) // TODO
		}
		if u.Time.After(toofarfuture) {
			return Errorf("time too far in future: %v", u.Time)
		}
	}
	return nil
}

func (u UnixTimestamp) Value() (interface{}, error) {
	if u.Time.IsZero() || u.Time.Before(zerotime) {
		return int64(0), nil
	}
	return FuncFrom(u.Time), nil
}

var NoCheckTimeScan bool
var zerotime = FuncTo(0)
var nineties, _ = time.Parse("2006-01-02", "1990-01-01")

// thousands of years in the future, to detect if someone uses milliseconds by accident
var toofarfuture = FuncTo(nineties.UnixMilli())
