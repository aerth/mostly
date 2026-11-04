package mllm_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aerth/mostly/mllm"
)

func TestMain(t *testing.M) {
	slog.SetDefault(slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true, ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				path := a.Value.Any().(*slog.Source)
				a.Value = slog.StringValue(fmt.Sprintf("%s:%d", filepath.Base(path.File), path.Line))
			}
			return a
		}}),
	)) // log json to stdout
	def := mllm.Default()

	// check for required env vars
	if err := def.ValidateConfig(false); err != nil {
		println("SKIPPING: mllm tests:", strings.ReplaceAll(err.Error(), "\n", " "))
		return
	}
	os.Exit(t.Run())
}

func TestExample(t *testing.T) {
	ctx := t.Context()
	var cr mllm.ChatResponse
	err := <-mllm.Default().StreamBG(ctx, mllm.Messages{{Role: "user", Content: "hi"}}, false, cr.Handler)
	println("Bot:", cr.Message.Content)
	if err != nil {
		t.Fatal(err)
	}
	println(cr.SummaryString())
}

func TestGenerateGetThink(t *testing.T) {
	ctx := t.Context()
	for _, thinkVal := range []any{false, "high", false} {
		t.Run(fmt.Sprintf("GenerateGet(think=%v)", thinkVal), func(t *testing.T) {
			client := mllm.Default()
			client.Writer = os.Stderr
			t1 := time.Now()
			got, err := client.
				WithTools(nil).Get(ctx, testMessageConvo1(), thinkVal)
			if err != nil {
				t.Fatal(err)
				return
			}
			t2 := time.Now()
			slog.Info("test results got", "think_value", thinkVal, "d", t2.Sub(t1).String())
			println(got.SummaryString())
		})
	}
}

func TestStream(t *testing.T) {
	ctx := t.Context()

	e := mllm.Default()
	e.Writer = os.Stderr
	err := e.Stream(ctx, testMessageConvo1(), false)
	if err != nil {
		t.Fatal("plain stream failed", err)
	}
}

func TestGetWithDefaultStreamHandler(t *testing.T) {
	var buf bytes.Buffer

	// Default() will have this writer as a stream handler
	client := mllm.Default()
	client.Writer = io.MultiWriter(&buf, os.Stderr) // stream generated output to stderr
	//mllm.HideThink = true
	ctx := t.Context()
	println("starting:")
	got, err := client.Get(ctx, testMessageConvo1(), false)
	if err != nil {
		t.Fatal(err)
	}
	println("finished:", "\n"+got.Message.Content)
	println(got.SummaryString())
	if buf.Len() == 0 {
		return
	}
	if got.Message.Content+"\n" != buf.String() { // stderr has newline after resp.Done
		t.Fatalf("wanted stderr to show same as Get, got: %s", buf.String())
	}
}

func testMessageConvo1() []mllm.Message {
	return []mllm.Message{
		{Role: "system", Content: "show a markdown code-fenced python implementation of the required function. no boilerplate/imports/examples/usage"},
		{Role: "user", Content: "from this golang: add(x int,y int) int"},
	}
}

// TestEmptyA ollama allows empty convo to start up the model. This library does not.
func TestEmptyA(t *testing.T) {
	e := mllm.Default()

	t.Run("empty streambg", func(t *testing.T) {
		err := <-e.StreamBG(t.Context(), nil, false)
		if !errors.Is(err, mllm.ErrNoConvo) {
			t.Fatal(err)
		}
	})
	t.Run("empty streambg", func(t *testing.T) {
		got, err := e.Get(t.Context(), nil, false)
		if !errors.Is(err, mllm.ErrNoConvo) {
			t.Fatal(err)
		}
		if !got.Message.IsEmpty() {
			t.Fatal("expected IsEmpty to be true for empty convo")
		}
	})
}
