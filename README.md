# mostly

## stackerr

Use `stackerr.Errorf("%w", err)` everywhere, and then `log.Printf("%+v", err)` for helpful error stack trace.

When using `slog` package, the trace is simplified to a single-line.

## superchan

Cancellable Channel Context [superchan](./superchan/)

```go
 import "github.com/aerth/mostly/superchan"
 var mainctx = superchan.NewMain(newmainctx, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM).(*superchan.Superchan[os.Signal])
 var _ context.CancelCauseFunc = mainctx.Cancel

```

## ncode

[encoding](./ncode/), twist arrays (`[]string -> []MyString -> []string`)

## anydb

store/fetch bbolt.DB helpers for nested buckets (path as `[][]byte` or `[]string`)

_(anydb was removed to keep go.mod empty)_

## httpserver

a cancellable, signal handling, httpserver+context combo for all kinds of web server programs

## flagpkg

for InverseBoolVar (eg: `--no-foo` to set foo to false)

## filters

for FilterCopy and FilterInPlace

## unixtimestamp

json marshaller for using unix timestamp instead of json string
