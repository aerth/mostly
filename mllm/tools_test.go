package mllm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/aerth/mostly/mllm"
)

type Tool = mllm.Tool

func init() {
	mllm.Flags()
}

func TestRandom(t *testing.T) {
	client := mllm.Default()
	ctx := t.Context()
	l := 3
	for i := range l {
		println("Part", i+1)
		_, err := client.Get(ctx, mllm.Messages{
			{Role: "system", Content: fmt.Sprintf("You are a story writer. For each topic given, reply with the first sentence of  part %d of %d of the short story.", i+1, l)},
			{Role: "user", Content: "A story about dragons"},
		}, false)
		if err != nil {
			t.Fatal(err)
		}
	}
}

// echoTool has no exec (fake tool)
var echoTool = Tool{
	Spec: json.RawMessage(`{"name": "echo_tester", "description": "Returns the input string exactly as provided.", "parameters": {"type": "object", "properties": {"input": {"type": "string", "description": "The text to be echoed"}}, "required": ["input"]}}`),
	Exec: nil,
}

// catTool has no exec (fake tool)
var catTool = Tool{Spec: json.RawMessage(`{"name": "cat_tester", "description": "Reads and returns the contents of a file.", "parameters": {"type": "object", "properties": {"path": {"type": "string", "description": "The path to the file"}}, "required": ["path"]}}`), Exec: nil}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err.Error())
	}
	return v
}

// catTool2 has exec func (still fake though)
var catTool2 = Tool{
	Spec: must(mllm.WrapToolSpec(catTool.Spec)), // same spec but wrapped
	Exec: func(ctx context.Context, args map[string]any) (any, error) {
		name := args["path"]
		val := "123ABC567"
		return fmt.Sprintf("Contents of %q:\n%s", name, val), nil
	},
}

func TestTool1(t *testing.T) {
	// model should be able to call a tool, consumer should be able to parse arguments
	t.Run("echo", func(t *testing.T) {
		client := mllm.Default()
		client.Writer = os.Stderr
		ctx := t.Context()
		client.Tools = []json.Marshaler{echoTool}
		think := false
		println("---")
		got, err := client.Get(ctx, mllm.Messages{
			{Role: "user", Content: "use the tool to say hello world"},
		}, think)
		println("---")
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Message.ToolCalls) == 0 {
			t.Fatal("expected tool calls")
		}
		// slog.Info("raw call", "raw", got.Message.ToolCalls[0])
		// parse args
		var call mllm.Call[struct {
			Input string `json:"input"`
		}]
		//		call.FromPlain()
		if true {
			return
		}
		err = json.Unmarshal(got.Message.ToolCalls[0], &call.Function.Arguments)
		if err != nil {
			t.Fatal("decode", err)
		}
		if call.Function.Name != "echo_tester" {
			t.Fatalf("expected echo_tester call, got %q", call.Function.Name)
		}
		if call.Function.Arguments.Input != "hello world" {
			t.Fatalf("expected 'hello world' got %q", call.Function.Arguments.Input)
		}
		slog.Info("got call", "call", call)
	})
	for _, realtool := range []Tool{catTool2, catTool} {
		// cat_true cat_false
		t.Run(fmt.Sprintf("cat_%v", realtool.Exec != nil), func(t *testing.T) {
			//		slog.Info("testing cat", "with_func", realtool.Exec != nil)
			ctx := t.Context()
			client := mllm.Default()
			client.StreamWriter.Writer = nil
			client.Writer = os.Stderr
			client.Options = &mllm.Options{
				Temperature: new(1.0),
			}
			client.Tools = []json.Marshaler{realtool}
			convo := mllm.Messages{
				{Role: "user", Content: "use the tool to read foo.txt, tell me the value it contains, no markdown"},
			}
			got, err := client.Get(ctx, convo, false)
			println("---")
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Message.ToolCalls) == 0 {
				t.Fatal("expected tool calls")
			}
			var call mllm.Call[struct{ Path string }]
			if err := call.FromRaw(got.Message.ToolCalls[0]); err != nil {
				println("raw:", string(got.Message.ToolCalls[0]))
				t.Fatalf("fromraw: %+v", err)
			}
			{
				if call.Function.Name != "cat_tester" {
					t.Fatalf("expected cat_tester call, got %q", call.Function.Name)
				}
				if call.Function.Arguments.Path != "foo.txt" {
					t.Fatalf("expected foo.txt, got %q", call.Function.Arguments.Path)
				}
				slog.Info("got call", "call", call)
			}

			var dummy any = "The foo.txt file says: 123ABC567"
			if realtool.Exec != nil {
				println("running dummy func")
				tt := realtool.Exec.(mllm.CallFuncV1) // TODO: exec switch
				dummy, err = tt(ctx, map[string]any{
					"path": call.Function.Arguments.Path, // TODO: other exec func sigs
				})
				if err != nil {
					t.Fatal("running tool exec func", err)
				}
			}
			toolout := call.ReplyTool(dummy)
			println("toolout:", toolout.Content)
			println("---")
			convo = append(convo, mllm.Messages{got.Message, toolout}...)
			//_ = json.NewEncoder(os.Stderr).Encode(convo)
			got, err = client.Get(ctx, convo, false)
			println("---")
			if err != nil {
				t.Fatal("could not reply to tool", "err", err)
			}
			if !strings.Contains(got.Message.Content, "123ABC567") {
				t.Fatal("expected to find the string in the file")
			}
		})
	}
}

func testerr(t *testing.T, err error) {
	if err != nil {
		debug.PrintStack()
		t.Fatalf("%+v", err)
	}
}

func TestToolName(t *testing.T) {
	if name := catTool.GetName(); name != "cat_tester" {
		t.Errorf("expected cat_tester got %q", name)
	}
	if name := catTool2.GetName(); name != "cat_tester" {
		t.Errorf("expected cat_tester got %q", name)
	}
}

func TestToolBasic(t *testing.T) {
	ctx := t.Context()
	realtool := catTool2
	if realtool.GetName() == "" {
		t.Fatalf("tool name failed")
	}
	client := mllm.Default()
	client.Writer = os.Stderr
	client.StreamWriter.Writer = nil
	client.Tools = []json.Marshaler{realtool}
	convo := mllm.Messages{
		{Role: "user", Content: "use the tool to read foo.txt, tell me the value it contains, no markdown"},
	}

	// 1. generate
	caller, err := client.Get(ctx, convo, false)
	if err != nil {
		t.Fatalf("%+v", err)
	}
	convo = append(convo, caller.Message) // IMPORTANT: add to convo for post-tool response

	if len(caller.Message.ToolCalls) == 0 {
		t.Fatal("llm did not choose to call tool function")
	}
	// 2. toolrunner parses args to route to a tool function
	// var call mllm.Call[map[string]any]

	/*
		var calls []
		index := 0
		for _, js := range caller.Message.ToolCalls {
			var call mllm.Call[json.RawMessage]
			if err := json.Unmarshal(js, &call); err != nil {
				println("tried json:", string(js))
				t.Fatalf("Unmarshal Call[string] %+v", err)
			}
			if len(calls) == 0 || index != call.Index {
				calls = append(calls, call.ToJSON())
				index = call.Index
				continue
			}
			l := len(calls)
			calls[l-1].Function.Arguments = append(calls[l-1].Function.Arguments, args...)
		}*/
	// do something if completed?
	//	println(len(caller.Message.ToolCalls), string(caller.Message.ToolCalls[0].Function.Arguments))
	// err = json.Unmarshal(caller.Message.ToolCalls[0], &call)
	// testerr(t, err)

	var call mllm.Call[struct{ Path string }]
	fmt.Printf("tried %d: %q", len(caller.Message.ToolCalls), string(caller.Message.ToolCalls[0]))
	err = call.FromRaw(caller.Message.ToolCalls[0])
	if err != nil {
		t.Fatal(err)
	}
	// 3. find tool based on name
	if call.Function.Name != catTool2.GetName() {
		t.Fatalf("llm didnt pick the right tool (wanted %q got %q)", catTool2.GetName(), call.Function.Name)
	}

	// 4. extract Exec func
	type ExecFunc = (func(ctx context.Context, args map[string]any) (any, error)) // your tool function signature
	toolexecfn := realtool.Exec.(ExecFunc)
	if call.Function.Arguments.Path != "foo.txt" {
		t.Fatalf("expected foo.txt but got %q", call.Function.Arguments.Path)
	}

	// 5. get tool output, make new reply message
	v, err := toolexecfn(ctx, call.Function.ArgsToMap())
	testerr(t, err)
	toolout := call.ReplyTool(v)
	convo = append(convo, toolout) // IMPORTANT: add to convo for post-tool response

	// 6. get response (which might contain tool calls)
	final, err := client.Get(ctx, convo, false)
	testerr(t, err)
	if len(final.Message.ToolCalls) != 0 {
		t.Fatal("llm did not get tool output")
	}

	// a tool runner might return
	// convo[l-3:]
	// or only the final message (l-1)
}

func TestTool_GetName(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		spec string // json
	}{
		{name: "cat_tester", spec: string(catTool.Spec)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: construct the receiver type.
			to := Tool{Spec: json.RawMessage(tt.spec)}
			got := to.GetName()
			if got != tt.name {
				t.Errorf("GetName() = %q, want %q", got, tt.name)
			}
		})
	}
}

func UnquoteJSON(v string) string {
	var x string
	if err := json.Unmarshal([]byte(v), &x); err != nil {
		slog.Error("unquote json string", "err", err)
		return v
	}
	return x
}
