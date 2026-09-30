package mllm_test

import (
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/aerth/mostly/mllm"
)

func init() {
	// NOTE: splitting stderr/stdout streams correctly (for jq) needs compiled test binary (see makefile)

	slog.SetDefault(slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true}),
	)) // log json to stdout

	mllm.DefaultWriter = os.Stderr // stream generated output to stderr
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

func TestGenerateGet(t *testing.T) {
	ctx := t.Context()
	for _, thinkVal := range []any{"high", false} {
		t1 := time.Now()
		gotFalse, err := mllm.Default().WithTools(nil).Get(ctx, testMessageConvo1(), thinkVal)
		if err != nil {
			t.Fatal(err)
			return
		}
		t2 := time.Now()
		slog.Info("test results got", "think_value", thinkVal, "d", t2.Sub(t1).String())
		println(gotFalse.SummaryString())
	}
}

func toJson(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

var _ = toJson

func TestStream(t *testing.T) {
	ctx := t.Context()
	err := mllm.Default().Stream(ctx, testMessageConvo1(), false)
	if err != nil {
		t.Fatal(err)
	}
}

func testMessageConvo1() []mllm.Message {
	return []mllm.Message{
		{Role: "system", Content: "show a markdown code-fenced python implementation of the required function. no boilerplate/imports/examples/usage"},
		{Role: "user", Content: "from this golang: add(x int,y int) int"},
	}
}
