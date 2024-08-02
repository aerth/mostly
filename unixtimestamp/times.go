package unixtimestamp

import (
	"encoding/binary"
	"strconv"
	"time"
)

// Endian is the default byte order for UnixTimestamp for MarshalBinary interface
var Endian = binary.LittleEndian

// UnixTimestamp is a time.Time that marshals to/from Unix timestamp (seconds)
// Use pointer (*UnixTimestamp) and `omitempty` in structs for JSON marshaling
// Use Wrap(t) or Now() to create a UnixTimestamp
type UnixTimestamp struct {
	time.Time
}

var zerotime = time.Unix(0, 0)

func (ut UnixTimestamp) MarshalJSON() ([]byte, error) {
	if ut.Time.After(zerotime) {
		return []byte(strconv.Itoa(int(time.Time(ut.Time).Unix()))), nil
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
	ut.Time = time.Unix(int64(unix), 0)
	return nil
}

// MarshalBinary uses Endian var, set Endian to binary.BigEndian if needed
func (ut UnixTimestamp) MarshalBinary() ([]byte, error) {
	var buf [8]byte
	if ut.Time.After(zerotime) {
		Endian.PutUint64(buf[:], uint64(ut.Time.Unix()))
	}
	return buf[:], nil
}

// UnmarshalBinary uses Endian var, set Endian to binary.BigEndian if needed
func (ut *UnixTimestamp) UnmarshalBinary(dat []byte) error {
	ut.Time = time.Unix(int64(Endian.Uint64(dat)), 0)
	if !ut.Time.After(zerotime) {
		ut.Time = time.Time{}
	}
	return nil
}

// New existing time.Time
func New(t time.Time) *UnixTimestamp {
	return &UnixTimestamp{t}
}

// Now returns a new UnixTimestamp for the current time
func Now() *UnixTimestamp {
	return New(time.Now())
}
