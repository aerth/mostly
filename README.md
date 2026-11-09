# mostly

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
 import "github.com/aerth/mostly/superchan"
 var mainctx = superchan.NewMain(newmainctx, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM).(*superchan.Superchan[os.Signal])
 var _ context.CancelCauseFunc = mainctx.Cancel

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

```go
 import "github.com/aerth/mostlymllm"
_, err := mllm.Default().Generate(ctx, "", "Write a short story about dragons.", false)
```

## httpserver

a cancellable, signal handling, httpserver+context combo for all kinds of web server programs

## flagpkg

`InverseBoolVar` (eg: `--no-foo` to set foo to false)

```go
	import "github.com/aerth/mostly/flagpkg"
	// example:
	cfg.ComplatePage = true
	flagpkg.InverseBoolVar(&cfg.CompletePage, "no-complete", cfg.CompletePage, "dont make whole html page")
```

## filters

for FilterCopy and FilterInPlace

## unixtimestamp

json marshaller for using unix timestamp instead of json string
