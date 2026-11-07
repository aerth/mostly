# mllm

tiny ai client package

## About

The `mllm` package is a lightweight, extensible LLM client library designed to simplify interactions with Large Language Models (LLMs) via providers such as llama.cpp or Ollama. It provides high-level abstractions for handling conversation messages, streaming responses (via a `StreamBG` method and configurable `StreamHandler` callbacks), and tool calling. The package simplifies environment-based configuration (Model, Host, API Key) and requires users to implement their own "tool runners" to execute logic based on the `Tool` specifications and `Call` arguments provided by the LLM.

## Environment Runtime Config

Reads these env vars (in order):

Model: "MLLM_MODEL", "OLLAMA_MODEL", "LLM_MODEL"
Host: "MLLM_HOST", "OLLAMA_HOST", "LLM_HOST"
API Key (optional): "MLLM_API_KEY", "OLLAMA_API_KEY", "LLM_API_KEY"

Use `DEBUG_LLM=1` to see JSON requests/responses

Host may be a full URL. Otherwise, `/v1/chat/completions` is added.

**Model and Host are required**.

Flags may target `mllm.DefaultOptions` to override Temperature, etc.

```go
flag.Float64Var(mllm.DefaultOptions.Temperature,
  "temperature", *mllm.DefaultOptions.Temperature, "temperature value for ai")
```

Programs may bypass the environment-based configuration by various means,
such as replacing mllm.Default, or creating a mllm.Client manually.

```go
apiurl , _ := url.Parse("https://ollama.home.arpa/v1/chat/completions")
client := mllm.E{
  Host: *apiurl,
  Model: "gemma4:12b",
  Writer: os.Stdout,
}
_, err := client.Generate(context.TODO(), "", "Tell funny joke")
if err != nil {
  log.Fatalln(err)
}
```

## Provider

Tested with llama.cpp:

```bash
export LLM_MODEL=gemma-4-E2B-it-Q4_K_M LLM_HOST=http://127.0.0.1:9931/v1/chat/completions LLM_API_KEY=some-key-here
```

Tested with ollama:

```bash
export LLM_MODEL=gemma4:12b LLM_HOST=http://127.0.0.1:11434/v1/chat/completions
```

## Usage

Typical usage for TUI would be (background) streaming with a custom StreamHandler to write into a view that renders markdown.

### Background

Use `StreamBG` to send the request, returning a `chan error` that is closed when stream is finished. A nil error is sent if no error occurred. The channel must be emptied.

TUI/GUI applications may use `StreamBG` with custom StreamHandler to stream the LLM output frames directly to a _view_ (possibly rendering markdown at intervals), and can access `cr.Done` to signal other threads that the generation is finished, to save and add the message to the "conversation".

```go
// simple orchestration loop

 client := mllm.Default() // uses env for Model and Host
 think := false
 e.Tools = mllm.Tools{searchTool,scrapeTool}

// convo is list of mllm.Message which might look familiar:
 convo := mllm.Messages{
  {Role: "system", Content: "You are a ..."}, 
  {Role: "user", Content: "what is the latest version of golang, and when will the next one be released?",
 }}

 // ChatResponse itself has a Handler method that is a StreamHandler func
 var chatResp mllm.ChatResponse
 var handlers []StreamHandler // stream handlers are called each frame/delta
 handlers = []StreamHandler{
   mllm.NewHandlerOut(os.Stderr, "THINKING:\n", "\nFINISHED THINKING\n"),
   chatResp.Handler,
   // your other handler for sending to View
 }
Chat:
 err := <- e.StreamBG(ctx, convo, think, handlers...)
 // add the "caller" to the conversation first
 convo = append(convo, chatResp.Message)
 // then, give "caller" to toolrunner, etc
 if len(chatResp.Message.ToolCalls) != 0 {
   // then, add toolout to convo
   toolout := runTools(ctx, chatResp.Message)
   convo = append(convo, toolout)
   goto Chat // go back to the generate step
 }
 finalMessage := chatResp.Message
```

See also: [test file](./mllm_test.go)

### Foreground

For non-streaming behavior, use `msg, err := client.Get(...)`

_Under the hood, we still use `"stream": true`, but our `Generate`, `Get`, and `GetMessage` functions do not return until the llm is finished (same with `Get` function)_

```go
 client := mllm.Default() // uses env for Model and Host
 client.Tools = nil // add tools to client
 think := false
 var resp mllm.Message
 var err error
 // Get/GetMessage accepts convo and think as arguments.
 cr, err := client.Get(ctx, messages, think)
 resp = cr.Message 
 // or, use GetMessage:
 resp, err = client.GetMessage(ctx, messages, think)

 // or, for first or only message, Generate can be used 
 // Generate accepts two string arguments (system, user):
 resp, err = client.Generate(ctx, "You are a helpful assistant.", "Send golang official documentation links", think)
}
```

See also: [test file](./mllm_test.go)

## StreamHandler

Although it is usually not necessary, a new StreamHandler is easy to make.
It might contain state. If it returns any error the stream will cut short.
A StreamHandler is called **very quickly**, and **will** lag the llm stream (see example below).

```go
import "github.com/aerth/mostly/mllm"

// as a function

func Dummy(cr mllm.ChatResponse) error {
 return nil
}

// as a method, stateful for adding each frame to it's own message copy
type X struct{Message mllm.Message, Ch chan mllm.Message}

func (x *X) Dummy(cr mllm.ChatResponse) error {
  x.Message = x.Message.Add(cr.Message) // combined
  if cr.Done {
 select {
 case x.Ch <- x.Message:
 default:
 return fmt.Errorf("GUI message channel is full")
 }
  }
 return nil
}
```

### Tools

In this package, a **Tool** contains an "Exec" function (who's signature is up to you), paired with a JSON "Spec" which is a `json.RawMessage`.
A tool **Call**, given by LLM, has "Arguments", (which may be an object or stringified object, or null or empty string).
This package does not run tools, or care what function signature you use as tool's Exec, and therefore you must implement your own "tool runner".

An example tool runner is [explainfile](../cmd/explainfile/main.go)

See also: [tool test file](./tools_test.go)

#### Tool Exec Funcs

Example tool function signatures might include (but are in no way limited to):

`func(ctx context.Context, args json.RawMessage) (string, error)`

`func(args map[string]any) (any, error)`

`func(ctx context.Context, args SomeArgumentType, other SessionData) (SomeResponseType, error)`

#### `call.ReplyTool`

The `call.ReplyTool` helper function (for a tool runner) to wrap a tool output into text for a new message to be added to the conversation.
It adds `ToolCallID` and `ToolName`, and JSON encodes the input for the new message's `Content`.

### Example Tool

Example `TimeTool` implemented in Go, using a `(ctx context.Context, args map[string]any) (string, error)` exec func signature.

The `"function":` wrapper is unnecessary as this package always ensures the spec is wrapped.

```go
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aerth/mostly/mllm"
)

var TimeTool = mllm.Tool{
	Spec: timeSpec,
	Exec: getTime,
}

var timeSpec = json.RawMessage(`{
  "type": "function",
  "function": {
    "name": "get_time",
    "description": "get current time",
    "parameters": {
      "type": "object",
      "properties": {
        "zone": {
          "type": "string"
        }
      },
      "required": ["zone"]
    }
  }
}`)

func getTime(ctx context.Context, args map[string]any) (string, error) {
	zone, ok := args["zone"].(string)
	if !ok {
		return "", errors.New("invalid zone")
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return "", fmt.Errorf("load location failed: %w", err)
	}
	return time.Now().In(loc).Format(time.RFC3339), nil
}
```
