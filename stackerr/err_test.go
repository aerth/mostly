package stackerr_test

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/aerth/mostly/stackerr"
)

// NOTE: these tests have line numbers
// [reserved line]
// NOTE: use utils_test.go so this file's imports change less frequently
var linestart = 2 // should stay 0 but is nice

func TestError(t *testing.T) {
	err := stackerr.Wrap(io.EOF) // prevline
	prevline := 20 + linestart
	got := fmt.Sprintf("%+v", err)
	logg.Printf("got: %s", got)
	if got != "EOF\n\tfrom stackerr_test.TestError (err_test.go:"+fmt.Sprint(prevline)+")" {
		t.Fatalf("expected line %d got %q", prevline, got)
	}
}

func TestError2a(t *testing.T) {
	err := stackerr.Errorf("%w", io.EOF)
	err = stackerr.Errorf("%w", err) // prevline
	if !errors.Is(err, io.EOF) {
		t.Fatal("wanted Is")
	}
	prevline := 31 + linestart
	got := fmt.Sprintf("%+v", err)
	logg.Printf("got: %s", got)
	want := "EOF\n\tfrom stackerr_test.TestError2a (err_test.go:" + fmt.Sprint(prevline) + ")" +
		"\n\tfrom stackerr_test.TestError2a (err_test.go:" + fmt.Sprint(prevline-1) + ")"
	if got != want {
		t.Fatalf("want\n%q\n got\n%q", want, got)
	}
}

func TestError2(t *testing.T) {
	err := stackerr.Wrap(io.EOF)
	err = stackerr.Wrap(err) // prevline
	if !errors.Is(err, io.EOF) {
		t.Fatal("wanted Is")
	}
	prevline := 47 + linestart
	got := fmt.Sprintf("%+v", err)
	logg.Printf("got: %s", got)
	want := "EOF\n\tfrom stackerr_test.TestError2 (err_test.go:" + fmt.Sprint(prevline) + ")" +
		"\n\tfrom stackerr_test.TestError2 (err_test.go:" + fmt.Sprint(prevline-1) + ")"
	if got != want {
		t.Fatalf("want\n%q\n got\n%q", want, got)
	}
}

func TestJson(t *testing.T) {
	err := stackerr.Wrap(io.EOF)
	err = stackerr.Wrap(err) // prevline
	if !errors.Is(err, io.EOF) {
		t.Fatal("wanted Is")
	}
	prevline := 64 + linestart
	_ = prevline
	js := jsonMarshal(err)
	logg.Println(string(js))
	// JSON uses [stackerr.StackError.MarshalText]
	want := `"EOF   from stackerr_test.TestJson (err_test.go:` + fmt.Sprint(prevline) + `)   from stackerr_test.TestJson (err_test.go:` + fmt.Sprint(prevline-1) + `)"`
	if string(js) != want {
		got := string(js)
		t.Fatalf("want\n%q\n got\n%q", want, got)
	}
	js = jsonMarshal(err.(*stackerr.StackError).St)
	logg.Printf("stack json: %s", string(js))
	if !strings.HasPrefix(string(js), `{"name":"stackerr_test.TestJson","file":"/home/username/src/mostly/stackerr/err_test.go","line":`+fmt.Sprint(prevline)+`,"time":`) {
		t.Fatalf("json did not")
	}
}

func TestMain(t *testing.M) {
	log.SetFlags(log.Flags() | log.Lshortfile)
	flag.Parse()
	outputwriter := func() io.Writer {
		if !testing.Verbose() {
			return io.Discard
		}
		return os.Stderr
	}()
	logg = log.New(outputwriter, "", log.Lshortfile)
	os.Exit(t.Run())
}

var logg = log.New(io.Discard, "", 0)
