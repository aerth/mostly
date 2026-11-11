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

// CallFuncV1 is completly optional. Any function signature may be used by a custom toolrunner.
type (
	CallFuncV1  = func(ctx context.Context, args map[string]any) (output any, err error)
	CallFuncV1B = func(ctx context.Context, args json.RawMessage) (output any, err error)
	CallFuncV1C = func(ctx context.Context, m Message, index int) (output any, err error)
)

// ToJSON ensures each tool is wrapped in a `"type": "function", "function":` wrapper.
//
// see [Tool.MarshalJSON]
func (t Tools) ToJSON() []json.Marshaler {
	a := make([]json.Marshaler, len(t))
	for i := range t {
		js, _ := WrapToolSpec(t[i].Spec)
		a[i] = json.RawMessage(js)
	}
	return a
}

// ByName get a Tool
func (t Tools) ByName(name string) Tool {
	for i := range t {
		if t[i].GetName() == name {
			return t[i]
		}
	}
	return Tool{}
}

type ctxkey string

const (
	ctxkeyCaller    = ctxkey("caller")
	ctxkeyCall      = ctxkey("call")
	ctxkeyCallIndex = ctxkey("call-index")
	ctxkeyClient    = ctxkey("client")
)

func WithCaller(ctx context.Context, caller Message) context.Context {
	return context.WithValue(ctx, ctxkeyCaller, caller)
}

func WithCall[T any](ctx context.Context, call Call[T]) context.Context {
	return context.WithValue(ctx, ctxkeyCall, call)
}

func WithCallIndex(ctx context.Context, index int) context.Context {
	return context.WithValue(ctx, ctxkeyCall, index)
}

func WithClient(ctx context.Context, client E) context.Context {
	return context.WithValue(ctx, ctxkeyClient, client)
}

func CtxCaller(ctx context.Context) (Message, bool) {
	x, ok := ctx.Value(ctxkeyCaller).(Message)
	return x, ok
}

func CtxCallIndex(ctx context.Context) (int, bool) {
	x, ok := ctx.Value(ctxkeyCallIndex).(int)
	return x, ok
}

func CtxCall[T any](ctx context.Context) (Call[T], bool) {
	x, ok := ctx.Value(ctxkeyCaller).(Call[T])
	return x, ok
}

func CtxClient(ctx context.Context) (E, bool) {
	x, ok := ctx.Value(ctxkeyClient).(E)
	return x, ok
}

// Run returns the raw tool output, such as `42` or `"foo"` or `{"foo": 42}`
//
// Tool Runner should wrap this in a [ReplyTool] or [ReplyToolOrError]
func (t Tools) Run(ctx context.Context, name string, args any) (output any, err error) {
	tool := t.ByName(name)
	if tool.Exec == nil { // a non-exec tool should not be handled by Run
		return nil, fmt.Errorf("tool %q not found", name)
	}
	return tool.Run(ctx, args)
}

// Run should be wrapped in a [ReplyTool] or [ReplyToolOrError]
func (t Tool) Run(ctx context.Context, args any) (output any, err error) {
	switch fn := t.Exec.(type) {
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
		case nil:
			m = nil
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
		case nil:
			m = nil
		case json.RawMessage:
			m = x
		default:
			return nil, fmt.Errorf("invalid args type %T", args)
		}
		return fn(ctx, m)
	case CallFuncV1C:
		caller, ok := CtxCaller(ctx)
		if !ok {
			return nil, fmt.Errorf("no caller in ctx")
		}
		index, _ := CtxCallIndex(ctx)
		return fn(ctx, caller, index) // TODO: multiple calls again
	}
	return nil, fmt.Errorf("no runner for sig %T", t.Exec)
}

// GetName extracts the name for finding the matching tool for a [Call]
//
// Used by [Tools.ByName]
func (t Tool) GetName() string {
	var x Name
	if err := json.Unmarshal(t.Spec, &x); err != nil {
		slog.Error("GetName: json decode tool spec failed", "err", err)
		return ""
	}
	name := x.GetName()
	if name == "" {
		println("check tool spec (has no name):", string(t.Spec))
	}
	return name
}

// Name finder (wrapped or unwrapped) in `"type":"function"`
// Caller should access [Name.GetName] method
//
// Used by [Tool.GetName] and [Tools.ByName]
type Name struct {
	Name     string   `json:"name,omitempty"`
	Function struct { // if wrapped
		Name string `json:"name,omitempty"`
	} `json:"function"`
}

// GetName of [Tool] function [Spec] or [Call]
func (n Name) GetName() string {
	if n.Function.Name != "" {
		return n.Function.Name
	}
	return n.Name
}

// Call is for a tool-runner to decode arguments.
// Callers may use Call[struct{Path string}] to decode `"arguments": {"path": "foo.txt"}`
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

// ToRaw converts back to a js arguments
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

// FromRaw attempts to decode from JSON
func (c *Call[T]) FromRaw(js json.RawMessage) error {
	if len(js) == 0 {
		return nil
	}
	isString := false
	if _, ok := any(c.Function.Arguments).(string); ok {
		isString = true
		//	println("STRING")
		//} else {
		//	println(fmt.Sprintf("%T NOT STRING", c.Function.Arguments))
	}
	if !isString && js[0] != '{' {
		var v string
		if err := json.Unmarshal(js, &v); err != nil {
			return fmt.Errorf("unwrap stage 1: %w", err)
		}
		js = json.RawMessage(v)
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
	if len(js) < 5 {
		return nil, nil
	}
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
		if x == "" {
			x = "(the tool had no output)"
		}
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
			Slog().Error("encoding tool output failed", "err", err, "typ", fmt.Sprintf("%T", val))
			v = fmt.Sprint(val)
		}
	}
	return Message{Role: "tool", ToolCallID: callID, ToolName: toolname, Content: v}
}
