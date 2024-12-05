// unixtimestamp package provides a time.Time wrapper that marshals to/from Unix timestamp (integer, seconds)
package unixtimestamp

import (
	"database/sql/driver"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

var Errorf = fmt.Errorf

// New existing time.Time
func New(t time.Time) *UnixTimestamp {
	return &UnixTimestamp{Time: t}
}

// Now returns a new UnixTimestamp for the current time
func Now() *UnixTimestamp {
	return New(time.Now().UTC()) // nil tz and remove monotonic clock
}

// Endian is the default byte order for UnixTimestamp for MarshalBinary interface
var Endian = binary.LittleEndian

// UnixTimestamp is a time.Time that marshals to/from Unix timestamp (seconds)
// Use pointer (*UnixTimestamp) and `omitempty` in structs for JSON marshaling
// Use Wrap(t) or Now() to create a UnixTimestamp
type UnixTimestamp = UnixTimestampNull

type UnixTimestampNotNull struct {
	UnixTimestamp
}
type UnixTimestampNull struct {
	time.Time
}

func (ut *UnixTimestamp) String() string {
	if ut == nil || ut.Time.IsZero() {
		return "(none)"
	}
	return ut.Time.String()
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
	var t time.Time
	switch x := v.(type) {
	case nil:
		t = time.Time{}
	case time.Time:
		t = x
	case int64:
		if x == 0 {
			t = time.Time{}
		} else {
			t = FuncTo(x)
		}
	default:
		return Errorf("unsupported type: %T", v)
	}
	u.Time = t

	return nil
}

// Value returns the time.Time value, regardless of zero time
//
// (does it work? no idea)
// func (u UnixTimestampNotNull) Value() (driver.Value, error) {
// 	log.Printf("UnixTimestampNotNull.Value: %v", u.Time)
// 	return u.Time, nil
// }

// Value is returns nil if zero time. Wrap with UnixTimestampNotNull for not null
func (u *UnixTimestamp) Value() (driver.Value, error) {
	if u == nil || u.Time.IsZero() {
		return nil, nil
	}
	return u.Time, nil
}

var NoCheckTimeScan bool
var zerotime = FuncTo(0)
var nineties = func() time.Time {
	t, err := time.Parse("2006-01-02", "1990-01-01")
	if err != nil {
		panic(err.Error())
	}
	return t
}()

// thousands of years in the future, to detect if someone uses milliseconds by accident
var toofarfuture = FuncTo(nineties.UnixMilli())

// for switching between seconds, milliseconds, and microseconds (json/txt marshal)

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

func toJson(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err.Error())
	}
	return string(b)
}
