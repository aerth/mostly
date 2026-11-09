# [mostly](https://github.com/aerth/mostly)

Collection of small packages.

## stackerr

Use `stackerr.Errorf("%w", err)` everywhere, and then `log.Printf("%+v", err)` for helpful error stack trace.

When using `slog` package, the trace is simplified to a single-line.

```go
err := stackerr.Errorf("%w", io.EOF)
slog.Error("reading file", "err", err)
```

## superchan

Cancellable Channel Context [superchan](./superchan/)

```go
 import (
 	"context"
 	"os"
 	"syscall"

 	"github.com/aerth/mostly/superchan"
 )

 var (
 	mainctx = superchan.NewMain(newmainctx,
 		syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM).(*superchan.Superchan[os.Signal])

 	_ context.CancelCauseFunc = mainctx.Cancel
    _ context.Context = mainctx
    _ context.Context = mainctx.Context // to pass only the ctx not a canceller
 )

```

## ncode

[encoding](./ncode/), twist arrays (`[]string -> []MyString -> []string`)

```go
	// example, turn strings to *url.URL
	var urls []*url.URL
	var err error
	urls, err = ncode.TwistParsePtr([]string{"https://google.com", "https://myspace.com"}, url.Parse)

	var envs []string
	envs, err = ncode.TwistParse([]string{"XDG_CONFIG_HOME", "HOME"}, os.Getenv)

```

## mllm

[tiny ai client](https://github.com/aerth/mostly/tree/master/mllm) for llm and embeddings

Works with llama.cpp and ollama, using `/v1/chat/completions` endpoint. The `Default` function loads a new client from environment (`LLM_MODEL`, `LLM_HOST`, `LLM_API_KEY`)

```go
package main

import (
	"context"

	"github.com/aerth/mostly/mllm"
)

func main() {
	ctx := context.TODO()
	_, err := mllm.Default().Generate(ctx, "[some system prompt]", "[some prompt]", false)
	if err != nil {
		println(err.Error())
	}
}
```

## httpserver

a cancellable, signal handling, httpserver+context combo for all kinds of web server programs

```go
package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/aerth/mostly/httpserver"
)

func main() {
	srv := httpserver.NewDefault()
	go func() {
		time.Sleep(time.Second * 10)
		srv.Cancel(fmt.Errorf("insert more coin"))
	}()
	srv.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "Hello!\n")
	})
	err := srv.ListenAndServeAll(":80", ":443", "cert.pem", "key.pem")
	println("the error is:", err.Error())
}
```

## flagpkg

`InverseBoolVar` (eg: `--no-foo` to set foo to false)

```go
	import "github.com/aerth/mostly/flagpkg"
	// example:
	cfg.ComplatePage = true
	flagpkg.InverseBoolVar(&cfg.CompletePage, "no-complete", cfg.CompletePage, "dont make whole html page")
```

## filters

For `FilterCopy` and `FilterInPlace`

```go
	// filter evens and odds different ways.
	// notice the Backing has two [7,9] after filters.FilterInPlace.
	nums := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	evenCopy := filters.FilterCopy(nums, func(n int) bool {
		return n%2 == 0
	})
	fmt.Println("Original:", nums)
	fmt.Println("Copy:", evenCopy)
	oddInPlace := filters.FilterInPlace(nums, func(n int) bool {
		return n%2 != 0
	})
	fmt.Println("InPlace:", oddInPlace)
	fmt.Println("Backing:", nums)
	// Output:
	// Original: [1 2 3 4 5 6 7 8 9 10]
	// Copy: [2 4 6 8 10]
	// InPlace: [1 3 5 7 9]
	// Backing: [1 3 5 7 9 6 7 8 9 10]
```

## journalwriter

```go
import (
	"log"

	"github.com/aerth/mostly/journalwriter"
)

func foo() {
	log.SetOutput(journalwriter.GetJournalOrStderr(journalwriter.PriInfo))
	log.Printf("hello systemd journal")
	_ = journalwriter.Send(msg, journalwriter.PriInfo, map[string]string{
		"SYSLOG_IDENTIFIER": "mostly", // instead of program_invocation_short_name(3)
	})
}
```
