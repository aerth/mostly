package unixtimestamp

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMicroseconds(t *testing.T) {
	var t1 = time.Now()
	// wow now its microseconds
	FuncFrom = FMilli
	FuncTo = TMilli
	buf, err := Now().MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	println("json:    ", string(buf))
	var y *UnixTimestamp
	json.Unmarshal(buf, &y)
	if y == nil {
		t.Fatalf("UnmarshalJSON: nil")
	}
	println("y:       ", y.String())
	if y.After(t1.Add(time.Second)) {
		t.Fatalf("After")
	}
	if y.Before(t1.Add(-time.Second)) {
		t.Fatalf("Before")
	}
}

func TestTimestamp(t *testing.T) {
	var t1 = time.Now()
	FuncFrom = FSeconds
	FuncTo = TSeconds
	var y *UnixTimestamp
	buf, err := Now().MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	println("json:    ", string(buf))
	json.Unmarshal(buf, &y)
	if y == nil {
		t.Fatalf("UnmarshalJSON: nil")
	}
	println("y:       ", y.String())
	if y.After(t1.Add(time.Second)) {
		t.Fatalf("After")
	}
	if y.Before(t1.Add(-time.Second)) {
		t.Fatalf("Before")
	}
}
