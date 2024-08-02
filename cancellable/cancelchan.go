package cancellable

import "context"

var _ Cancellable = (*cancellableChan[uint])(nil)
var _ Chan[uint] = (*cancellableChan[uint])(nil)

var CHANBUFSIZE = 1000 // default buffer size for NewChan

// Chan holds a context and channel and cancelfunc.
type Chan[T any] interface {
	Cancellable
	UpdatesChan() <-chan T // Returns chan for receiving. choose 1: only use if NOT using Updates()
	Updates() []T          // Collects recent sent-to-chan. choose 1: only use if NOT using UpdatesChan()
	Updates2() []T         // TODO: benchmark this against Updates()
	CloseChan()            // manual, optional: only when no more sending to chan
	Ch() chan<- T          // sender only
}

// NewChanFrom for type T (must Cancel, optionally CloseChan)
func NewChanFrom[T any](parent context.Context, cancelfunc context.CancelCauseFunc) Chan[T] {
	return WrapChan[T](parent, cancelfunc, nil)
}

// WrapChan for pre-existing channel. All fields may be nil for behavior like NewChanFrom
func WrapChan[T any](parent context.Context, cancelfunc context.CancelCauseFunc, ch chan T) Chan[T] {
	if parent == nil && cancelfunc == nil {
		parent, cancelfunc = context.WithCancelCause(context.Background())
	}
	if parent == nil || cancelfunc == nil {
		panic("WrapChan: parent and cancelfunc must be both nil or both non-nil")
	}
	if ch == nil {
		ch = make(chan T, CHANBUFSIZE)
	}
	return &cancellableChan[T]{
		cancellable: newFrom(parent, cancelfunc),
		ch:          ch,
	}
}

// NewChan for type T (must Cancel, optionally CloseChan)
func NewChan[T any](parent context.Context) Chan[T] {
	ctx, cancel := context.WithCancelCause(parent)
	return NewChanFrom[T](ctx, cancel)
}

type cancellableChan[T any] struct {
	*cancellable
	ch chan T
}

func (c *cancellableChan[T]) UpdatesChan() <-chan T {
	return c.ch
}

func (c *cancellableChan[T]) CloseChan() {
	close(c.ch)
}

func (c *cancellableChan[T]) Updates() []T {
	l := len(c.ch)
	var updates []T = make([]T, l)
	for i := 0; i < l; i++ {
		u := <-c.ch
		updates[i] = u
	}
	return updates
}

// TODO benchmark this against Updates()
func (c *cancellableChan[T]) Updates2() []T {
	var updates []T
	for {
		select {
		case u := <-c.ch:
			updates = append(updates, u)
		default:
			if len(updates) == 0 {
				return nil
			}
			return updates
		}
	}
}

func (c *cancellableChan[T]) Ch() chan<- T {
	return c.ch
}
