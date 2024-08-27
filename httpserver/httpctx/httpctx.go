// httpctx package provides context keys for http server handlers.
package httpctx

import (
	"context"
	"crypto/tls"
	"net"
)

type contextKey string

const KListener contextKey = "listener" // for assigning listener to context
const KUUID contextKey = "uuid"         // for assigning UUID (RequestID) to context
const KConn contextKey = "conn"         // for assigning net.Conn to context

// GetUUID returns unique Request ID for this request (not user ID)
func GetUUID(ctx context.Context) int {
	if v := ctx.Value(KUUID); v != nil {
		return v.(int)
	}
	return 0
}

// GetTCPConn same as GetConn but uses different type assertion
//
// Example:
//
//	var _, isTcp = httpctx.GetTCPConn(ctx)
func GetTCPConn(ctx context.Context) (*net.TCPConn, bool) {
	if v := ctx.Value(KConn); v != nil {
		x, ok := v.(*net.TCPConn)
		return x, ok
	}
	return nil, false
}

// GetTLSConn same as GetConn but uses different type assertion
//
// Example:
//
//	var _, isTls = httpctx.GetTLSConn(ctx)
func GetTLSConn(ctx context.Context) (*tls.Conn, bool) {
	if v := ctx.Value(KConn); v != nil {
		x, ok := v.(*tls.Conn)
		return x, ok
	}
	return nil, false
}

// GetConn see also GetTCPConn and GetTLSConn
func GetConn(ctx context.Context) net.Conn {
	if v := ctx.Value(KConn); v != nil {
		x, ok := v.(net.Conn)
		_ = ok
		return x
	}
	return nil
}

// GetListener (net.Listener) returns listener assigned to context
func GetListener(ctx context.Context) net.Listener {
	if v := ctx.Value(KListener); v != nil {
		x, ok := v.(net.Listener)
		_ = ok
		return x
	}
	return nil
}

func GetAny[T any](ctx context.Context, tag any) (T, bool) {
	if v := ctx.Value(tag); v != nil {
		x, ok := v.(T)
		return x, ok
	}
	var v T // if ptr, nil
	return v, false
}
