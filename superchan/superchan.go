package superchan

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/aerth/mostly/cancellable"
	"github.com/aerth/mostly/flagpkg"
)

var Log = log.Default()

// CHANBUFSIZE is the size of the channel buffer (cancellable package)
func SetChanSize(size int) {
	cancellable.CHANBUFSIZE = size
}

var MakeSignalError = func(sig os.Signal) error {
	return fmt.Errorf("caught sig: %v", sig)
}

// CancelBeforeDefer determines if the context is cancelled before running deferred funcs.
//
// Default is true so incoming signals will act the same as Cancel(err).
// (context is finished while deferred funcs are running)
var CancelBeforeDefer = true

// UseGoroutineDefer changes the way deferred funcs are run when a Superchan is finished.
//
// Default is true because it often safer to run deferred funcs in goroutines (we handle panic).
// See DeferFirst and DeferLast.
var UseGoroutineDefer = true

// Flags adds flags to the flag package for advanced superchan configuration at runtime.
func Flags() {
	flagpkg.InverseBoolVar(&CancelBeforeDefer, "defer-first", CancelBeforeDefer, "server: cancel last")
	flagpkg.InverseBoolVar(&UseGoroutineDefer, "defer-ordered", UseGoroutineDefer, "server: run deferred funcs in sequence")

}

// Superchan handles signals, is a cancellable.Chan[os.Signal] with defer funcs
//
// Use as main context, for example:
//
// var mainctx = New(context.Background(), os.Interrupt, syscall.SIGTERM)
//
//	func main() {
//	  <-mainctx.Done()
//	  log.Fatalln(context.Cause(mainctx))
//	}
type Superchan[T any] struct {
	cancellable.Chan[T]
	deferfuncs            []func() // starts as non-nil empty array
	deferlast, deferfirst func()
}

type Main = Superchan[os.Signal]

// DoneWaitSeconds is the number of seconds to wait for deferred funcs to finish after context is done
// Only used in Wait() function.
var MaxWaitDuration = time.Second * 5

func (s *Superchan[T]) IsDead() bool {
	return s.deferfuncs == nil // only happens after rundeferred finishes
}

// Wait (blocks) for context to be cancelled, then run deferred funcs.
//
// Prefer instead using s.DeferLast(wg.Done) with external waitgroup.
func (s *Superchan[T]) Wait() error {
	err := s.Chan.Wait()
	t1 := time.Now()
	for time.Since(t1) < MaxWaitDuration { // wait up to X extra seconds for deferred funcs to wrap up
		if s.IsDead() {
			return err
		}
		time.Sleep(time.Second / 100)
	}
	Log.Printf("warn: shutdown timed out after %s", time.Since(t1))
	return err
}

// Defer a function to run when the context is cancelled. See CancelBeforeDefer.
//
// # Could be a call to shutdown an http server, for example
//
// Ordering: funcs added later are run first (see DeferLast for a single lastfunc)
func (s *Superchan[T]) Defer(f ...func()) {
	if s.Err() != nil {
		panic("cannot defer after cancel")
	}
	//s.deferfuncs = append(s.deferfuncs, f...)
	for _, ff := range f {
		s.deferfuncs = append([]func(){ff}, s.deferfuncs...)
	}
}

// DeferFirst is called first after context is finished.
//
// Could be a call to http.Shutdown, for example
func (s *Superchan[T]) DeferFirst(f func()) {
	if s.Err() != nil {
		panic("cannot defer after cancel")
	}
	if s.deferfirst != nil {
		panic("deferfirst already set")
	}
	s.deferfirst = f
}

// DeferLast is called last after context is finished.
//
// Could be a call to wg.Done, for example
func (s *Superchan[T]) DeferLast(f func()) {
	if s.Err() != nil {
		panic("cannot defer after cancel")
	}
	if s.deferlast != nil {
		panic("deferlast already set")
	}
	s.deferlast = f
}

func (s *Superchan[T]) GetDeferred() []func() {
	return s.deferfuncs
}

func (s *Superchan[T]) SetDeferred(f []func()) {
	s.deferfuncs = f
}

// rundeferred, called ONCE from New gofunc, runs all deferred funcs in the order they were added.
func (s *Superchan[T]) rundeferred() {
	if s.IsDead() {
		panic("rundeferred called twice")
	}
	//Log.Printf("running deferred funcs: parallel=%v", UseGoroutineDefer)
	var wg sync.WaitGroup
	caller := func(fn func()) {
		fn() // call directly
	}
	if UseGoroutineDefer { // run deferred funcs in goroutines
		caller = func(fn func()) {
			wg.Add(1)
			go func() {
				defer wg.Done() // last deferred
				defer func() {
					if r := recover(); r != nil {
						Log.Printf("error in deferred func (panic): %v", r)
					}
				}()
				fn()
			}()
		}
	}
	// run deferred funcs, (first, the rest, then last)
	if s.deferfirst != nil {
		caller(s.deferfirst)
	}
	wg.Wait() // noop if not parallel
	for _, f := range s.deferfuncs {
		caller(f)
	}
	wg.Wait() // noop if not parallel
	if s.deferlast != nil {
		caller(s.deferlast)
	}
	wg.Wait() // noop if not parallel
	s.deferlast = nil
	s.deferfirst = nil
	s.deferfuncs = nil
}

// New Superchan for signal handling with defer funcs and context cancellation
// one goroutine is started to handle signals calling cancel and defer funcs, see CancelBeforeDefer.
//
// Note: New uses cancellable.CHANBUFSIZE (1000) for the channel buffer size (see SetChanSize).
//
// For type assert, use x.(*superchan.Superchan[os.Signal])
func NewMain(parent context.Context, signals ...os.Signal) cancellable.Cancellable {
	if len(signals) == 0 {
		panic("superchan: no signals provided")
	}
	chctx := NewRaw[os.Signal](parent)
	signal.Notify(chctx.Ch(), signals...)
	if chctx.Err() != nil { // rare
		return chctx
	}
	go mainSignalHandler(chctx)
	return chctx
}

func mainSignalHandler(chctx *Superchan[os.Signal]) {
	defer func() {
		signal.Stop(chctx.Ch())
		close(chctx.Ch())
	}()
	select {
	case <-chctx.Done(): // someone else cancelled the ctx
		chctx.rundeferred()
	case in := <-chctx.UpdatesChan(): // signal caught, lets cancel the context
		if CancelBeforeDefer {
			chctx.Cancel(MakeSignalError(in))
			chctx.rundeferred()
		} else {
			chctx.rundeferred()
			chctx.Cancel(MakeSignalError(in))
		}
	}
}

// New Superchan for processing a channel, with defer funcs and cancellation.
//
// Reads from the channel and calls the handler func for every update. Send to chctx.Ch(), cancel with chctx.Cancel(err).
//
// The handler func should return nil error unless you want the context cancelled, (stopping the reader loop).
func New[T any](parent context.Context, handler func(context.Context, T) error, parallel bool) *Superchan[T] {
	if handler == nil {
		panic("superchan: no handler provided")
	}
	chctx := NewRaw[T](parent)
	go func() {
		for chctx.Err() == nil {
			select {
			case <-chctx.Done(): // someone else cancelled the ctx
				chctx.rundeferred()
			case in := <-chctx.UpdatesChan(): // signal caught, lets cancel the context
				if parallel {
					go func() {
						if err := handler(chctx, in); err != nil {
							chctx.Cancel(err) // should break loop and run deferred funcs
						}
					}()
				} else {
					if err := handler(chctx, in); err != nil {
						chctx.Cancel(err) // should break loop and run deferred funcs
					}
				}
			}

		}
	}()
	return chctx
}

func NewRaw[T any](parent context.Context) *Superchan[T] {
	return &Superchan[T]{Chan: cancellable.NewChan[T](parent), deferfuncs: []func(){}} // non-nil
}

// New Double Superchan for processing a channel, with handler func, defer funcs and cancellation.
//
// Send to chctx.Ch() and cancel everything with chctx.Cancel(err).
//
// Unhandled packets will be sent to chctx2.Ch(), but if buffer is full they are dropped.
//
// Send to first, Handler processes, then Receive from second if handler returns non-nil. (chctx2.UpdateChan())
// Reads from the channel and calls the handler func for every update.
//
// The handler func can be nil, or a function called before sending to chctx2.
func NewDouble[T any](parent context.Context, handler func(context.Context, **T) error, parallel bool) (*Superchan[T], *Superchan[T]) {
	if handler == nil {
		panic("superchan: no handler provided")
	}
	chctx := NewRaw[T](parent)
	chctx2 := NewRaw[T](chctx)
	go func() {
		defer func() {
			close(chctx.Ch())
			close(chctx2.Ch())
		}()
		for chctx.Err() == nil {
			select {
			case <-chctx.Done(): // someone else cancelled the ctx
				chctx.rundeferred()
				return
			case <-chctx2.Done(): // someone else cancelled the ctx
				chctx.rundeferred()
				return
			case in := <-chctx.UpdatesChan(): // signal caught, lets cancel the context
				if parallel {
					go handledoublein(chctx, chctx2, in, handler)
				} else {
					handledoublein(chctx, chctx2, in, handler)
				}
			}

		}
	}()
	return chctx, chctx2
}

// may or may not be called in a goroutine
func handledoublein[T any](chctx *Superchan[T], chctx2 *Superchan[T], in T, handler func(context.Context, **T) error) {
	if chctx.Err() != nil || chctx2.Err() != nil {
		return
	}
	var input *T = &in
	var out = &input
	if handler != nil {
		if err := handler(chctx2, out); err != nil {
			chctx.Cancel(err) // should break loop and run deferred funcs
			return
		}
	}
	if *out == nil {
		//	println("handled by handler")
		return
	}
	//println("output handler: ", **out)
	select {
	case <-chctx.Done():
	case <-chctx2.Done():
	case chctx2.Ch() <- **out:
	default:
		//println("dropped") // buffer is full
	}
}
