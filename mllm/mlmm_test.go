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
	// NOTE: needs compiled test binary to split streams correctly
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{})))
	mllm.DefaultWriter = os.Stderr
}

func TestGenerateGet(t *testing.T) {
	ctx := t.Context()
	for _, thinkVal := range []any{"high", false} {
		slog.Info("get", "think", thinkVal)
		t1 := time.Now()
		gotFalse, err := mllm.Default().Get(ctx, testMessageConvo1(), thinkVal)
		if err != nil {
			t.Fatal(err)
			return
		}
		t2 := time.Now()
		//	_ = mllm.StderrHandler(gotFalse)
		slog.Info("test results got", "think_value", thinkVal, "d", t2.Sub(t1).String())
		println(gotFalse.SummaryString())
	}
}

func toJson(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

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
