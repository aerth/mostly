package mllm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"
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

func (tt Tools) ToJSON() []json.Marshaler {
	a := make([]json.Marshaler, len(tt))
	for i := range tt {
		js, _ := WrapToolSpec(tt[i].Spec)
		a[i] = json.RawMessage(js)
	}
	return a
}

func (tt Tools) ByName(name string) Tool {
	for i := range tt {
		if tt[i].GetName() == name {
			return tt[i]
		}
	}
	return Tool{}
}

// Run returns the raw tool output, such as `42` or `"foo"` or `{"foo": 42}`
//
// Tool Runner should wrap this in a [ReplyTool] or [ReplyToolOrError]
func (tt Tools) Run(ctx context.Context, name string, args any) (output any, err error) {
	tool := tt.ByName(name)
	if tool.Exec == nil {
		return nil, fmt.Errorf("tool not found")
	}
	switch fn := tool.Exec.(type) {
	case CallFuncV1:
		var m map[string]any
		switch x := args.(type) {
		case map[string]any:
			m = x
		case json.RawMessage:
			m = map[string]any{}
			if err := json.Unmarshal(x, &m); err != nil {
				return nil, fmt.Errorf("arg not object: %w", err)
			}
		default:
			return nil, fmt.Errorf("invalid args type %T", args)
		}
		return fn(ctx, m)
	case CallFuncV1B:
		var m json.RawMessage
		switch x := args.(type) {
		case map[string]any:
			js, err := json.Marshal(x)
			if err != nil {
				return nil, err
			}
			m = js
		case json.RawMessage:
			m = x
		default:
			return nil, fmt.Errorf("invalid args type %T", args)
		}
		return fn(ctx, m)
	}
	return nil, fmt.Errorf("no runner for sig %T", tool.Exec)
}

// CallFuncV1 is completly optional and any may be used by a custom toolrunner
type (
	CallFuncV1  = func(ctx context.Context, args map[string]any) (output any, err error)
	CallFuncV1B = func(ctx context.Context, args json.RawMessage) (output any, err error)
)

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
//
// See [Call.UnmarshalJSON]
type Call[T any] struct {
	ID       string      `json:"id,omitempty"`
	Index    int         `json:"index"`
	Type     string      `json:"type,omitempty"`
	Function Function[T] `json:"function"`
}
type Function[T any] struct {
	Name      string `json:"name"`
	Arguments T      `json:"arguments,omitempty"`
}

func (c Call[T]) ToRaw() Call[json.RawMessage] {
	js, err := json.Marshal(c.Function.Arguments)
	if err != nil {
		panic(err.Error())
	}
	return Call[json.RawMessage]{
		Function: Function[json.RawMessage]{
			Name:      c.Function.Name,
			Arguments: js,
		},
		ID:    c.ID,
		Index: c.Index,
		Type:  c.Type,
	}
}

var ddebug = false

func (c *Call[T]) FromRaw(js json.RawMessage) error {
	if len(js) == 0 {
		return nil
	}
	isString := false
	if _, ok := any(c.Function.Arguments).(string); ok {
		isString = true
		//	println("STRING")
	} else {
		println(fmt.Sprintf("%T NOT STRING", c.Function.Arguments))
	}
	if !isString && js[0] != '{' {
		if ddebug {
			println("FromRaw given stringified object")
		}
		var v string
		if err := json.Unmarshal(js, &v); err != nil {
			return fmt.Errorf("unwrap stage 1: %w", err)
		}
		js = json.RawMessage(v)
	} else {
		if ddebug {
			fmt.Printf("FromRaw given a object direct %T: %s\n", c.Function.Arguments, string(js))
		}
	}
	err := json.Unmarshal(js, &c)
	if err != nil {
		return fmt.Errorf("FromRaw unmarshal Call[%T]: %+v", c.Function.Arguments, err)
	}
	return nil
}

func (f Function[T]) ToJSON() json.RawMessage {
	b, err := json.Marshal(f)
	if err != nil {
		panic(err.Error())
	}
	return b
}

func (f Function[T]) ArgsToMap() map[string]any {
	b, err := json.Marshal(f.Arguments)
	if err != nil {
		panic(err.Error())
	}

	m := map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		panic(err.Error())
	}
	return m
}

func (f Function[T]) ArgsToJSON() json.RawMessage {
	b, err := json.Marshal(f.Arguments)
	if err != nil {
		panic(err.Error())
	}
	return b
}

func (c *Call[T]) ToJSON() json.RawMessage {
	b, err := json.Marshal(c)
	if err != nil {
		panic(err.Error())
	}
	return b
}

func (c *Call[T]) FromPlain(from Call[json.RawMessage]) error {
	c.Function.Name = from.Function.Name
	c.ID = from.ID
	c.Index = from.Index
	if err := json.Unmarshal(from.Function.Arguments, &c.Function.Arguments); err != nil {
		return Errorf("FromPlain: %w", err)
	}
	return nil
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
		return fmt.Errorf("unmarshal plain call: %w", err)
	}
	isString := fmt.Sprintf("%T", c.Function.Arguments) == "string"
	// case: "arguments": "{\"foo\": \"bar\"}"
	if !isString && len(rawcall.Function.Arguments) > 2 && rawcall.Function.Arguments[0] == '"' && rawcall.Function.Arguments[1] == '{' {
		// unquote
		var z string
		err = json.Unmarshal(rawcall.Function.Arguments, &z)
		if err != nil {
			return err
		}
		if len(z) == 0 || z[0] != '{' {
			return fmt.Errorf("unhandled arguments")
		}
		// now: "arguments": {"foo": "bar"}
		rawcall.Function.Arguments = json.RawMessage(z)
	}
	*c = Call[T]{
		ID:    rawcall.ID,
		Index: rawcall.Index,
		Type:  "function",
	}
	c.Function.Name = rawcall.Function.Name
	// decode args
	// case: "arguments": {"foo": "bar"}
	var args T
	if len(rawcall.Function.Arguments) != 0 {
		err = json.Unmarshal(rawcall.Function.Arguments, &args)
		if err != nil {
			debug.PrintStack()
			return fmt.Errorf("unmarshal inner args %T: %w", args, err)
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

func ReplyToolOrError[T any](call Call[T], val any, err error) Message {
	if err != nil {
		return ReplyTool(call, err)
	}
	return ReplyTool(call, val)
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
