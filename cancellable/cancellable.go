package cancellable

import "context"

// Cancellable is a context.Context that provides a Cancel func.
type Cancellable interface {
	context.Context
	Cancel(err error)            // stop listening and close connection. does not close chan.
	GetContext() context.Context // for interface compatibility (returns underlying Context, not self)
	Wait() error                 // wait for context to be done, return underlying error cause
}

// New cancellable (must Cancel to prevent context leak)
func New(parent context.Context) Cancellable {
	ctx, cancel := context.WithCancelCause(parent)
	return NewFrom(ctx, cancel)
}

// NewFrom wraps existing context and cancelfunc to provide same interface as New
func NewFrom(the context.Context, cancelfunc context.CancelCauseFunc) Cancellable {
	return newFrom(the, cancelfunc)
}

type cancellable struct {
	context.Context
	cancel context.CancelCauseFunc
}

// new cancellable context ptr, used by derived types that embed cancellable
func newFrom(the context.Context, cancelfunc context.CancelCauseFunc) *cancellable {
	return &cancellable{
		Context: the,
		cancel:  cancelfunc,
	}
}

func (c *cancellable) Cancel(err error) {
	if c.cancel == nil {
		panic("cancellable: not initialized")
	}
	c.cancel(err)
}
func (c *cancellable) GetContext() context.Context {
	return c.Context
}
func (c *cancellable) Wait() error {
	<-c.Done()
	return context.Cause(c)
}
