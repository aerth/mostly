# mllm

tiny ai client

## Environment Runtime Config

Tested with llama.cpp:

```bash
export LLM_MODEL=gemma-4-E2B-it-Q4_K_M LLM_HOST=http://127.0.0.1:9931/v1/chat/completions LLM_API_KEY=some-key-here
```

Tested with ollama:

```bash
export LLM_MODEL=gemma4:26b LLM_HOST=http://127.0.0.1:11434/v1/chat/completions
```


## Usage

Typical usage for TUI would be a background stream with a custom StreamHandler to write into a view that renders markdown.

### Background

Use `StreamBG` to send the request, returning a `chan error` that is closed when stream is finished.

```go
 client := mllm.Default() // uses env for Model and Host
 think := false
 var chatResp mllm.ChatResponse // has a Handler method that is a StreamHandler func
 var handlers []StreamHandler // stream handlers are called each frame/delta
 handlers = []StreamHandler{
   mllm.NewHandlerOut(os.Stderr, "THINKING:\n", "\nFINISHED THINKING\n"),
   chatResp.Handler,
   // some other handler if needed
 }
 err := <- e.StreamBG(ctx, messages, think, handlers...)
}
```

### Foreground

For non-streaming behavior, use `msg, err := client.Get(...)`

```go
 client := mllm.Default() // uses env for Model and Host
 think := false
 var resp mllm.Message
 var err error
 resp, err = client.GetMessage(ctx, messages, think)
}
```

See also: [test file](./mllm_test.go)

## StreamHandler

Although it is not necessary, a new StreamHandler is easy to make:

```go
import "github.com/aerth/mostly/mllm"

// as a function

func Dummy(cr mllm.ChatResponse) error {
	return nil
}

// as a method
type x struct{}

func (x) Dummy(cr mllm.ChatResponse) error {
	return nil
}

```

## ChatMessage

ChatMessage is an example multi-participant chat message. Most users of this package will use a plain Message instead, creating their own wrapper for persistence if needed. ChatMessage, unlike other parts of this package, is subject to change for now.
