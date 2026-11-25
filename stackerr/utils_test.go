package stackerr_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/aerth/mostly/stackerr"
)

var (
	Err2         = errors.New("E2")
	Err3         = errors.New("E3")
	ErrBaseError = fmt.Errorf("base error")
)

// [stackerr.StackError.Format]
// [stackerr.FuncCallerInfo.String]

func getError1() error {
	return stackerr.Errorf("the %w", ErrBaseError)
}

func getError2() error {
	err := getError1()
	return stackerr.Errorf("oh no:%w %w", Err2, err)
}

func getError3() error {
	err := getError2()
	return stackerr.Errorf("uh oh:%w %w", Err3, err)
}

func getError3w() error {
	err := getError3()
	return stackerr.Wrap(err)
}

func TestError3(t *testing.T) {
	err, err2, err3 := getError3(), stackerr.Wrap(getError2()), getError3w()
	if !errors.Is(err, ErrBaseError) {
		t.Fatal("not wrapped1")
	}
	if !errors.Is(err2, ErrBaseError) {
		t.Fatal("not wrapped2")
	}
	if !errors.Is(err3, ErrBaseError) {
		t.Fatal("not wrapped3")
	}
	if !errors.Is(err2, Err2) || errors.Is(err2, Err3) {
		t.Fatal("not wrapped2b")
	}
	if !errors.Is(err3, Err3) || !errors.Is(err3, Err2) {
		t.Fatal("not wrapped3b")
	}
	logg.Printf("good err1: %+v", err)
	logg.Printf("good err2: %+v", err2)
}

func jsonMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err.Error())
	}
	return b
}
