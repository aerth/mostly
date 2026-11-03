package mllm

import (
	"encoding/json"
	"fmt"
)

type Tool struct {
	Spec json.RawMessage
	Exec any `json:"-"`
}

type RawToolCall = Call[json.RawMessage]

// Call is for a tool-runner to decode arguments. Callers may use Call[struct{Path string}] to decode `"arguments": {"path": "foo.txt"}`
//
// It handles the case where Arguments json object is passed as a json string.
type Call[T any] struct {
	ID       string `json:"id"`
	Index    int    `json:"index"`
	Type     string `json:"type"` // "function" always?
	Function struct {
		Name      string `json:"name"`
		Arguments T      `json:"arguments,omitempty"`
	} `json:"function"`
}

func (t Tool) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"type": "function", "function": t.Spec})
}

func (c *Call[T]) UnmarshalJSON(b []byte) error {
	type plain Call[json.RawMessage]
	var x plain
	err := json.Unmarshal(b, &x)
	if err != nil {
		return err
	}
	// case: "arguments": "{\"foo\": \"bar\"}"
	if len(x.Function.Arguments) > 2 && x.Function.Arguments[0] == '"' && x.Function.Arguments[1] == '{' {
		// unquote
		var z string
		err = json.Unmarshal(x.Function.Arguments, &z)
		if err != nil {
			return err
		}
		x.Function.Arguments = json.RawMessage(z)
	}
	*c = Call[T]{
		ID:    x.ID,
		Index: x.Index,
		Type:  x.Type,
	}
	c.Function.Name = x.Function.Name

	// decode args
	// case: "arguments": {"foo": "bar"}
	var args T
	if len(x.Function.Arguments) != 0 {
		err = json.Unmarshal(x.Function.Arguments, &args)
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
