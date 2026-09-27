package stackerr_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/aerth/mostly/stackerr"
)

var ErrBaseError = fmt.Errorf("base error")

// [stackerr.StackError.Format]
// [stackerr.FuncCallerInfo.String]

func getError1() error {
	return stackerr.Errorf("the %w", ErrBaseError)
}

func getError2() error {
	err := getError1()
	return stackerr.Errorf("oh no: %w", err)
}

func getError3() error {
	err := getError2()
	return stackerr.Errorf("uh oh: %w", err)
}

func getError3w() error {
	err := getError3()
	return stackerr.Wrap(err)
}

func TestError3(t *testing.T) {
	err, err2 := getError3(), stackerr.Wrap(getError2())
	logg.Printf("err1: %+v", err)
	logg.Printf("err2: %+v", err2)
}

func jsonMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err.Error())
	}
	return b
}
