package mllm_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aerth/mostly/mllm"
	"github.com/aerth/mostly/vector256"
)

func TestMain(t *testing.M) {
	slog.SetDefault(slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true}),
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

	for i := range first {
		if !compareFloat(first[i], got[i]) {
			t.Fatalf("v %d: %v != %v", i, got[i], first[i])
		}
	}
	if false && !slices.Equal(first, got[:10]) { // TODO: it broke
		t.Fatalf("unexpected output: %#v", got[:10])
	}
}

func compareFloat[T ~float32 | ~float64](x, y T) bool {
	const epsilon = 1e-4
	diff := x - y
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}

func TestEmbed2(t *testing.T) {
	ctx := t.Context()
	x, err := mllm.Default().Embed(ctx, mllm.NewNomicQuery("testing test tester"))
	if err != nil {
		println(string(err.(mllm.ErrorWithResponse).Body()))
		t.Fatal(err)
	}
	if x == nil {
		t.Fatal(err)
	}
	println("SUMMARY")
	_ = x.Summary(t.Output())
	println("---")
	got := x.GetEmbeddings()
	if len(got) != vector256.VarLen {
		t.Fatalf("%d != %d", len(got), vector256.VarLen)
	}
	println("got:", fmt.Sprintf("%#v ... %d total", got[:10], vector256.VarLen))
	first := []vector256.F{
		0.03676727, 0.06071676, -0.29565915, -0.017700996, 0.11540128,
		-0.036267255, 0.04523779, -0.062040724, 0.007202007, -0.017709982,
	}
	for i := range first {
		if !compareFloat(first[i], got[i]) {
			t.Fatalf("v %d: %v != %v", i, got[i], first[i])
		}
	}
	if true {
		return
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
	for i := range first {
		if !compareFloat(first[i], got[i]) {
			t.Fatalf("v %d: %v != %v", i, got[i], first[i])
		}
	}
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
		t.Fatal(err)
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

var echoTool = mllm.Tool{
	Spec: json.RawMessage(`{"name": "echo_tester", "description": "Returns the input string exactly as provided.", "parameters": {"type": "object", "properties": {"input": {"type": "string", "description": "The text to be echoed"}}, "required": ["input"]}}`),
	Exec: nil,
}

func TestTool1(t *testing.T) {
	client := mllm.Default()
	client.Writer = os.Stderr
	// model should be able to call a tool, consumer should be able to parse arguments
	t.Run("echo", func(t *testing.T) {
		ctx := t.Context()
		client.Tools = []json.Marshaler{echoTool}
		got, err := client.Get(ctx, mllm.Messages{
			{Role: "user", Content: "use the tool to say hello world"},
		}, "low")
		if err != nil {
			t.Fatal(err)
		}
		println("---")
		if len(got.Message.ToolCalls) == 0 {
			t.Fatal("expected tool calls")
		}
		// slog.Info("raw call", "raw", got.Message.ToolCalls[0])
		// parse args
		var call mllm.Call[*struct {
			Input string `json:"input"`
		}]
		err = json.Unmarshal(got.Message.ToolCalls[0], &call)
		if err != nil {
			t.Fatal("decode", err)
		}
		if call.Function.Name != "echo_tester" {
			t.Fatalf("expected echo_tester call, got %q", call.Function.Name)
		}
		slog.Info("got call", "call", call)
	})
}
