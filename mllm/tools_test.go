package mllm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/aerth/mostly/mllm"
)

// echoTool has no exec (fake tool)
var echoTool = mllm.Tool{
	Spec: json.RawMessage(`{"name": "echo_tester", "description": "Returns the input string exactly as provided.", "parameters": {"type": "object", "properties": {"input": {"type": "string", "description": "The text to be echoed"}}, "required": ["input"]}}`),
	Exec: nil,
}

// catTool has no exec (fake tool)
var catTool = mllm.Tool{Spec: json.RawMessage(`{"name": "cat_tester", "description": "Reads and returns the contents of a file.", "parameters": {"type": "object", "properties": {"path": {"type": "string", "description": "The path to the file"}}, "required": ["path"]}}`), Exec: nil}

// catTool2 has exec func (still fake though)
var catTool2 = mllm.Tool{
	Spec: json.RawMessage(`{"name": "cat_tester", "description": "Reads and returns the contents of a file.", "parameters": {"type": "object", "properties": {"path": {"type": "string", "description": "The path to the file"}}, "required": ["path"]}}`),
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
	t.Run("cat", func(t *testing.T) {
		for _, realtool := range []mllm.Tool{catTool2, catTool} {
			slog.Info("testing cat", "with_func", realtool.Exec != nil)
			ctx := t.Context()
			client := mllm.Default()
			client.Writer = os.Stderr
			client.Tools = []json.Marshaler{realtool}
			convo := mllm.Messages{
				{Role: "user", Content: "use the tool to read foo.txt, tell me the value it contains, no markdown"},
			}
			got, err := client.Get(ctx, convo, false)
			if err != nil {
				t.Fatal(err)
			}
			println("---")
			if len(got.Message.ToolCalls) == 0 {
				t.Fatal("expected tool calls")
			}
			var call mllm.Call[struct{ Path string }]
			{
				// slog.Info("raw call", "raw", got.Message.ToolCalls[0])
				// parse args
				err = json.Unmarshal(got.Message.ToolCalls[0], &call)
				if err != nil {
					t.Fatal("decode", err)
				}
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
				tt := realtool.Exec.(CallFuncV1) // TODO: exec switch
				dummy, err = tt(ctx, map[string]any{
					"path": call.Function.Arguments.Path, // TODO: other exec func sigs
				})
				if err != nil {
					t.Fatal("running tool exec func", err)
				}
			}
			toolout := call.ReplyTool(dummy)
			convo = append(convo, mllm.Messages{got.Message, toolout}...)
			//_ = json.NewEncoder(os.Stderr).Encode(convo)
			got, err = client.Get(ctx, convo, false)
			if err != nil {
				t.Fatal("could not reply to tool", "err", err)
			}
			if !strings.Contains(got.Message.Content, "123ABC567") {
				t.Fatal("expected to find the string in the file")
			}
		}
	})
}

type CallFuncV1 = func(ctx context.Context, args map[string]any) (output any, err error)

func ReplyTool[T any](call mllm.Call[T], val any) mllm.Message {
	toolname := call.Function.Name
	callID := call.ID
	var v string
	switch x := val.(type) {
	case string:
		v = x
	case json.RawMessage:
		v = string(x)
	case fmt.Stringer:
		v = x.String()
	default:
		js, err := json.Marshal(val)
		if err == nil {
			v = string(js)
		} else {
			mllm.Slog().Error("encoding tool output failed", "err", err)
			v = fmt.Sprint(val)
		}
	}
	return mllm.Message{Role: "tool", ToolCallID: callID, ToolName: toolname, Content: v}
}
