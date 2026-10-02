package mllm_test

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/aerth/mostly/mllm"
	"github.com/aerth/mostly/vector256"
)

func init() {
	// NOTE: splitting stderr/stdout streams correctly (for jq) needs compiled test binary (see makefile)

	slog.SetDefault(slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true}),
	)) // log json to stdout
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

func TestEmbed1(t *testing.T) {
	ctx := t.Context()
	x, err := mllm.Default().Embed(ctx, mllm.NewEmbed("testing test tester"))
	if err != nil {
		t.Fatal(err)
	}
	if x == nil {
		t.Fatal(err)
	}
	_ = x.Summary(t.Output())
	got := x.GetEmbeddings()
	if len(got) != vector256.VarLen {
		t.Fatalf("%d != %d", len(got), vector256.VarLen)
	}
	println("got:", fmt.Sprintf("%v ... %d total", got[:3], vector256.VarLen))
	first := vector256.Vector256{0.03836852, 0.056748904, -0.27613422, -0.004759476, 0.11951496, -0.03993982, 0.05664575, -0.031644862, 0.01586234, -0.03084815}
	if !slices.Equal(first, got[:10]) {
		t.Fatalf("unexpected output: %#v", got[:10])
	}
}

func TestEmbed2(t *testing.T) {
	ctx := t.Context()
	x, err := mllm.Default().Embed(ctx, mllm.NewNomicQuery("testing test tester"))
	if err != nil {
		t.Fatal(err)
	}
	if x == nil {
		t.Fatal(err)
	}
	_ = x.Summary(t.Output())
	got := x.GetEmbeddings()
	if len(got) != vector256.VarLen {
		t.Fatalf("%d != %d", len(got), vector256.VarLen)
	}
	println("got:", fmt.Sprintf("%v ... %d total", got[:3], vector256.VarLen))
	first := []float32{
		0.03676727, 0.06071676, -0.29565915, -0.017700996, 0.11540128,
		-0.036267255, 0.04523779, -0.062040724, 0.007202007, -0.017709982,
	}
	if !slices.Equal(first, got[:10]) {
		t.Fatalf("unexpected output: %#v", got[:10])
	}
}

func TestEmbed3(t *testing.T) {
	ctx := t.Context()
	x, err := mllm.Default().Embed(ctx, mllm.NewNomicDocument("testing test tester"))
	if err != nil {
		t.Fatal(err)
	}
	if x == nil {
		t.Fatal(err)
	}
	_ = x.Summary(t.Output())
	got := x.GetEmbeddings()
	if len(got) != vector256.VarLen {
		t.Fatalf("%d != %d", len(got), vector256.VarLen)
	}
	println("got:", fmt.Sprintf("%v ... %d total", got[:3], vector256.VarLen))
	first := vector256.Vector256{0.046110913, 0.032962788, -0.20280331, 0.012861654, 0.1151968, -0.0126432115, 0.0051027983, -0.07082936, -0.01349577, -0.021741517}
	if !slices.Equal(first, got[:10]) {
		t.Fatalf("unexpected output: %#v", got[:10])
	}
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

func TestStream(t *testing.T) {
	ctx := t.Context()
	err := mllm.Default().Stream(ctx, testMessageConvo1(), false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetWithDefaultStreamHandler(t *testing.T) {
	old := mllm.DefaultWriter
	var buf bytes.Buffer
	mllm.DefaultWriter = io.MultiWriter(&buf, os.Stderr) // stream generated output to stderr
	t.Cleanup(func() {
		mllm.DefaultWriter = old
	})
	ctx := t.Context()
	println("starting:")
	got, err := mllm.Default().Get(ctx, testMessageConvo1(), false)
	if err != nil {
		t.Fatal(err)
	}
	println("finished:", "\n"+got.Message.Content)
	println(got.SummaryString())
	if got.Message.Content != buf.String()[:buf.Len()-1] { // stderr has newline after resp.Done
		t.Fatalf("wanted stderr to show same as Get, got: %s", buf.String())
	}
}

func testMessageConvo1() []mllm.Message {
	return []mllm.Message{
		{Role: "system", Content: "show a markdown code-fenced python implementation of the required function. no boilerplate/imports/examples/usage"},
		{Role: "user", Content: "from this golang: add(x int,y int) int"},
	}
}
