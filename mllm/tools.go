package mllm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// Tool has JSON spec and Exec func
//
// Your toolrunner can parse args and pass to Exec how you want to.
type Tool struct {
	Spec     // json.RawMessage     // Spec, contains name
	Exec any `json:"-"`
}
type Spec = json.RawMessage

type Tools []Tool

func (tt Tools) ByName(name string) Tool {
	for i := range tt {
		if tt[i].GetName() == name {
			return tt[i]
		}
	}
	return Tool{}
}

// CallFuncV1 is completly optional and any may be used by a custom toolrunner
type CallFuncV1 = func(ctx context.Context, args map[string]any) (output any, err error)

func (t Tool) GetName() string {
	var x Name
	if err := json.Unmarshal(t.Spec, &x); err != nil {
		slog.Error("GetName: json decode tool spec failed", "err", err)
	}
	name := x.GetName()
	if name == "" {
		println("check tool spec (has no name):", string(t.Spec))
	}
	return name
}

type Name struct {
	Name     string `json:"name,omitempty"` // "function" always?
	Function struct {
		Name string `json:"name,omitempty"`
	} `json:"function"`
}

func (n Name) GetName() string {
	if n.Function.Name != "" {
		return n.Function.Name
	}
	return n.Name
}

type RawToolCall = Call[json.RawMessage]

// Call is for a tool-runner to decode arguments. Callers may use Call[struct{Path string}] to decode `"arguments": {"path": "foo.txt"}`
//
// It handles the case where Arguments json object is passed as a json string.
type Call[T any] struct {
	ID       string `json:"id,omitempty"`
	Index    int    `json:"index"`
	Function struct {
		Name      string `json:"name"`
		Arguments T      `json:"arguments,omitempty"`
	} `json:"function"`
}

func WrapToolSpec(js json.RawMessage) ([]byte, error) {
	if isWrapped(js) {
		return json.Marshal(js)
	}
	return json.Marshal(map[string]any{"type": "function", "function": js})
}

func (t Tool) MarshalJSON() ([]byte, error) {
	return WrapToolSpec(t.Spec)
}

func isWrapped(rawMessage json.RawMessage) bool {
	return bytes.Contains(rawMessage, []byte(`"function":`))
}

func (c *Call[T]) UnmarshalJSON(b []byte) error {
	type Plain Call[json.RawMessage]
	var rawcall Plain
	err := json.Unmarshal(b, &rawcall)
	if err != nil {
		return fmt.Errorf("unmarshal call: %w", err)
	}
	// case: "arguments": "{\"foo\": \"bar\"}"
	if len(rawcall.Function.Arguments) > 2 && rawcall.Function.Arguments[0] == '"' && rawcall.Function.Arguments[1] == '{' {
		// unquote
		var z string
		err = json.Unmarshal(rawcall.Function.Arguments, &z)
		if err != nil {
			return err
		}
		// now: "arguments": {"foo": "bar"}
		rawcall.Function.Arguments = json.RawMessage(z)
	}
	*c = Call[T]{
		ID:    rawcall.ID,
		Index: rawcall.Index,
	}
	c.Function.Name = rawcall.Function.Name
	// decode args
	// case: "arguments": {"foo": "bar"}
	var args T
	if len(rawcall.Function.Arguments) != 0 {
		err = json.Unmarshal(rawcall.Function.Arguments, &args)
		if err != nil {
			return err
		}
	}
	c.Function.Arguments = args
	return nil
}

func (c Call[T]) ReplyTool(val any) Message {
	return ReplyTool(c, val)
}

func (c Call[T]) ReplyToolOrError(val any, err error) Message {
	if err != nil {
		return ReplyTool(c, err)
	}
	return ReplyTool(c, val)
}

func ReplyTool[T any](call Call[T], val any) Message {
	toolname := call.Function.Name
	callID := call.ID
	var v string
	switch x := val.(type) {
	case error:
		v = fmt.Sprintf("Tool returned error: %+v", x)
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
		} else { // TODO: non stringer non marshalable ...
			Slog().Error("encoding tool output failed", "err", err)
			v = fmt.Sprint(val)
		}
	}
	return Message{Role: "tool", ToolCallID: callID, ToolName: toolname, Content: v}
}
