// Package cancellable combines Context, CancelCauseFunc with Wait
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

// CancellableImplV1 implementation
type CancellableImplV1 struct {
	context.Context
	cancel context.CancelCauseFunc
}

// new cancellable context ptr, used by derived types that embed cancellable
func newFrom(the context.Context, cancelfunc context.CancelCauseFunc) *CancellableImplV1 {
	return &CancellableImplV1{
		Context: the,
		cancel:  cancelfunc,
	}
}

// Cancel calls the underlying cancel func.
//
// Implements [Cancellable] and is [context.CancelCauseFunc]
func (c *CancellableImplV1) Cancel(err error) {
	if c.cancel == nil {
		panic("cancellable: not initialized, error: " + err.Error())
	}
	c.cancel(err)
}

// GetContext implements [Cancellable]
func (c *CancellableImplV1) GetContext() context.Context {
	return c.Context
}

// Wait waits for underlying c.Context to be finished, returning Cause
func (c *CancellableImplV1) Wait() error {
	<-c.Done()
	return context.Cause(c)
}
