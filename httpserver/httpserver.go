package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/aerth/mostly/httpserver/httpctx"
	"github.com/aerth/mostly/superchan"
	"github.com/pkg/errors"
	"golang.org/x/exp/rand"
)

// HttpServer handles signals, use as main context
type HttpServer struct {
	*http.Server
	*superchan.Superchan[os.Signal]
	*http.ServeMux // at the bottom of all middleware
	*Config

	//
	entrypoint      func(http.Handler) http.Handler
	homehandler     http.HandlerFunc
	notfoundhandler http.HandlerFunc
	signalshandled  []os.Signal
	shutdownfunc1   func() // called before http shutdown
	shutdownfunc    func() // called after http shutdown
	refreshfunc     func(s *HttpServer) error
}

// Config is only for convenience, used by your application and middlewares
type Config struct {
	BaseURL string `json:"base_url"`
}

// called after Refresh() is completed, before Refresh() returns.
func (h *HttpServer) SetRefreshFunc(f func(s *HttpServer) error) {
	h.refreshfunc = f
}

// SetShutdownFunc to run after http server is shutdown
func (h *HttpServer) SetShutdownFunc(f func()) {
	h.shutdownfunc = f
}

// DeferLast override because httpserver occupies superchan.DeferLast
//
// f will be called AFTER all other deferred funcs, before ListenAndServeAll returns.
//
// is persistent across server.Refresh() calls, but will be replaced if called again.
func (h *HttpServer) DeferLast(f func()) {
	h.SetShutdownFunc(f)
}

// DeferFirst override because httpserver occupies superchan.DeferFirst
//
// f will be called BEFORE http server shutdown begins
func (h *HttpServer) DeferFirst(f func()) {
	h.shutdownfunc1 = f
}

func (c *Config) GetBaseURL() string {
	return c.BaseURL
}

// UUIDFunc may be replaced (for example, ordered UUIDs)
var UUIDFunc = func(c net.Conn) int {
	return rand.Intn(1000) + 1000
}

// New creates an httpserver (http+https) that closes on cancellation or signal (SIGINT, SIGHUP etc)
// After New, set ErrorLog and routing (Handle, SetHomeHandler, SetNotFoundHandler), then run ListenAndServeAll.
// Caller MUST NOT Handle("/") as it is reserved for the home/notfound combo handler.
// Will panic if handler is a servemux that already handles "/" path.
func New(ctx context.Context, routes *http.ServeMux, signals ...os.Signal) *HttpServer {
	if routes == nil {
		routes = http.NewServeMux()
	}
	var (
		chctx = superchan.NewMain(ctx, signals...).(*superchan.Superchan[os.Signal])
		x     = &HttpServer{
			Server:          buildserver(chctx, routes),
			Superchan:       chctx,
			Config:          &Config{},
			notfoundhandler: DefaultNotFoundHandler,
			entrypoint:      nil,
			homehandler:     nil,
			ServeMux:        routes,
			signalshandled:  signals,
		}
	)
	x.HandleFunc("/", x.basehandler)
	return x
}

// at the bottom of all middleware for home/notfound
func (s *HttpServer) basehandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" && s.homehandler != nil {
		s.homehandler(w, r)
		return
	}
	s.notfoundhandler(w, r)
}

func basectxfn(basectx context.Context) func(net.Listener) context.Context {
	return func(listener net.Listener) context.Context {
		return context.WithValue(basectx, httpctx.KListener, listener)
	}
}

var IdleTimeout = time.Second * 2

func connctxfun(ctx context.Context, c net.Conn) context.Context { // get conn
	ctx = context.WithValue(ctx, httpctx.KUUID, UUIDFunc(c))
	return context.WithValue(ctx, httpctx.KConn, c)
}
func buildserver(basectx context.Context, routes http.Handler) *http.Server {
	return &http.Server{
		Addr:                         "", // set by ListenAndServeAll
		Handler:                      routes,
		ReadTimeout:                  5 * time.Second,
		ReadHeaderTimeout:            5 * time.Second,
		WriteTimeout:                 10 * time.Second,
		IdleTimeout:                  IdleTimeout,
		MaxHeaderBytes:               1 << 20,
		ConnContext:                  connctxfun,
		ConnState:                    nil,
		BaseContext:                  basectxfn(basectx),
		TLSConfig:                    nil,
		DisableGeneralOptionsHandler: false,
		ErrorLog:                     nil,
		TLSNextProto:                 nil,
	}
}

// InsertMiddleware into the http server (if calling SwapServeMux, this must be called after)
//
// Ordering: handlers added later are called first.
func (s *HttpServer) InsertMiddleware(middleware ...func(http.Handler) http.Handler) {
	if s.Server.Handler == nil {
		panic("InsertMiddleware: no handler set, out of order?")
	}
	for _, m := range middleware {
		if m == nil {
			panic("InsertMiddleware: nil middleware provided")
		}
		s.Server.Handler = m(s.Server.Handler)
	}
}

// DefaultNotFoundHandler simple json 404
var DefaultNotFoundHandler = func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": "not found",
		"code":  http.StatusNotFound,
	})
}

// SetEntry is like InsertMiddleware but is inserted automatically at ListenAndServeAll time
func (s *HttpServer) SetEntryMiddleware(entrypoint func(http.Handler) http.Handler) {
	s.entrypoint = entrypoint
}

func (s *HttpServer) SetHomeHandler(h http.HandlerFunc) {
	s.homehandler = h
}

func (s *HttpServer) SetNotFoundHandler(h http.HandlerFunc) {
	s.notfoundhandler = h
}

// SwapServeMux with a new one (do this BEFORE calling InsertMiddleware)
//
// Replacing with a NewServeMux is not recommended, as it will not have the default NotFoundHandler.
func (s *HttpServer) SwapServeMux(mux *http.ServeMux) {
	if s.Server.Handler != s.ServeMux {
		panic("SwapServeMux: already added middleware, cannot swap")
	}
	s.ServeMux = mux
	s.Server.Handler = mux
	mux.HandleFunc("/", s.basehandler)
}

// ShutdownServer with timeout
func ShutdownServer(server *http.Server, timeout time.Duration) {
	// use a magic number to avoid extra allocations
	if server.IdleTimeout == 123123 {
		panic("httpserver: already shutdown")
	}
	server.IdleTimeout = 123123
	if server.ErrorLog != nil {
		server.ErrorLog.Printf("httpserver: shutting down")
	}
	short, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := server.Shutdown(short); err != nil {
		if server.ErrorLog != nil {
			server.ErrorLog.Printf("httpserver shutdown error: %v", err)
		}
	}
	// server is no longer listening.
	// the above server.Shutdown() is now running registered OnShutdown funcs.
}

// RegisterOnShutdown registers a function to call on underlying [http.Server.Shutdown].
//
// also see the much more useful: Defer(func()) and DeferLast(func())
//
// This can be used to gracefully shutdown connections that have
// undergone ALPN protocol upgrade or that have been hijacked.
// This function should start protocol-specific graceful shutdown,
// but should not wait for shutdown to complete.
func (s *HttpServer) RegisterOnShutdown(f func()) {
	s.Server.RegisterOnShutdown(f)
}

func (s *HttpServer) shutdown() {
	ShutdownServer(s.Server, 5*time.Second)
}
func (s *HttpServer) ListenAndServe() error {
	return fmt.Errorf("wrong function: use ListenAndServeAll")
}
func (s *HttpServer) ListenAndServeTLS(string, string) error {
	return fmt.Errorf("wrong function: use ListenAndServeAll")
}

// ListenAndServeAll starts the http server (http+https) and blocks until done.
// It will return an error if the server is cancelled or encounters an error during startup.
// After returning, Refresh() can be called before calling again
func (s *HttpServer) ListenAndServeAll(httpAddr string, httpsAddr string, cert, key string) error {
	if s.Err() != nil {
		return fmt.Errorf("httpserver: already cancelled: %v", s.Err())
	}
	// check params
	if httpAddr == "" && httpsAddr == "" {
		return fmt.Errorf("httpserver: no listen addresses provided")
	}
	if cert != "" && key == "" {
		return fmt.Errorf("httpserver: cert and key must be set together")
	}
	if key != "" && cert == "" {
		return fmt.Errorf("httpserver: cert and key must be set together")
	}
	if key != "" && httpsAddr == "" {
		return fmt.Errorf("httpserver: key and cert set, but no httpsAddr")
	}
	if key != "" {
		if _, err := os.Stat(key); err != nil {
			return fmt.Errorf("httpserver: key file not found: %v", err)
		}
		if _, err := os.Stat(cert); err != nil {
			return fmt.Errorf("httpserver: cert file not found: %v", err)
		}
	}
	// set entrypoint if exists
	if s.entrypoint != nil {
		s.Server.Handler = s.entrypoint(s.Server.Handler)
		s.entrypoint = nil // only once, even across refresh
	}
	s.listenAndServe(httpAddr, httpsAddr, cert, key)
	return context.Cause(s)
}

// OneClosesBoth is a global setting to close both of the http+https stack when one of them closes
var OneClosesBoth = true

func (s *HttpServer) ServeJson(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
func (s *HttpServer) ServeJsonIndent(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func (s *HttpServer) serveHttps(httpsAddr string, cert, key string, deferfunc func()) {
	defer deferfunc()
	if OneClosesBoth {
		defer s.Cancel(fmt.Errorf("https listener died"))
	}
	s.Addr = httpsAddr
	if s.ErrorLog != nil {
		s.ErrorLog.Printf("https server: starting https://%s", s.Addr)
	}
	err := s.Server.ListenAndServeTLS(cert, key)
	if s.ErrorLog == nil {
		log.Printf("wtf: %v", err)
		return
	}
	if err != nil && err != http.ErrServerClosed {
		s.ErrorLog.Println("critical error https server:", err)
	} else {
		s.ErrorLog.Printf("https server: no longer listening: %v", context.Cause(s))
	}
}

func (s *HttpServer) serveHttp(httpAddr string, deferfunc func()) {
	defer deferfunc()
	if OneClosesBoth {
		defer s.Cancel(fmt.Errorf("http listener died"))
	}
	s.Addr = httpAddr
	if s.ErrorLog != nil {
		s.ErrorLog.Printf("http server: starting http://%s", s.Addr)
	}
	err := s.Server.ListenAndServe()
	if s.ErrorLog == nil {
		return
	}
	if err != nil && err != http.ErrServerClosed {
		s.ErrorLog.Println("critical error https server:", err)
	} else {
		s.ErrorLog.Printf("https server: no longer listening: %v", context.Cause(s))
	}
}
func (s *HttpServer) listenAndServe(httpAddr string, httpsAddr string, cert, key string) {
	if httpAddr == "" && httpsAddr == "" {
		panic("listenAndServe: no listen addresses provided")
	}
	var wg sync.WaitGroup
	wg.Add(1) // wg: superchan DeferLast

	s.Superchan.DeferFirst(func() {
		if s.shutdownfunc1 != nil {
			s.shutdownfunc1()
		}
		s.shutdown() // shutdown http server (calls registered shutdown funcs)
	})
	s.Superchan.DeferLast(func() { // something else to wait for
		if s.shutdownfunc != nil {
			s.shutdownfunc()
		}
		wg.Done()
	})
	if key != "" && cert != "" && httpsAddr != "" {
		wg.Add(1) // wg: https enabled
		go s.serveHttps(httpsAddr, cert, key, wg.Done)
		time.Sleep(time.Second / 2) // race: wait for https to start to reuse for http server
	}
	if httpAddr != "" {
		wg.Add(1) // wg: http enabled
		go s.serveHttp(httpAddr, wg.Done)
	}
	wg.Wait()
}

// Refresh ONLY after closing the server (resets channel, context, reuses ServeMux)
// Will panic if called before server is closed.
// ONLY returns error if refreshfunc returns an error, in which case it cancels the context.
//
// If using Refresh(), check error before adding superchan.Defer functions.
func (s *HttpServer) Refresh(newmainctx context.Context) error {
	if s.Err() == nil {
		panic("httpserver: cannot refresh, no error")
	}
	if newmainctx == nil {
		newmainctx = context.Background()
	}
	old := s.Server
	s.Superchan = superchan.NewMain(newmainctx, s.signalshandled...).(*superchan.Superchan[os.Signal])
	s.Server = buildserver(s.Superchan, s.Server.Handler)
	copyHttpServer(s.Server, old)
	if s.refreshfunc != nil {
		if err := s.refreshfunc(s); err != nil {
			s.Cancel(errors.Wrap(err, "refresh"))
			return err
		}
	}
	return nil
}

// retain (most) customizations.
// ignores Handler, IdleTimeout, BaseContext
func copyHttpServer(s, s2 *http.Server) {
	s.Addr = s2.Addr
	//s.Handler = s2.Handler
	s.ReadTimeout = s2.ReadTimeout
	s.ReadHeaderTimeout = s2.ReadHeaderTimeout
	s.WriteTimeout = s2.WriteTimeout
	//s.IdleTimeout = s2.IdleTimeout
	s.MaxHeaderBytes = s2.MaxHeaderBytes
	s.ConnContext = s2.ConnContext
	s.ConnState = s2.ConnState
	//s.BaseContext = s2.BaseContext
	s.TLSConfig = s2.TLSConfig
	s.DisableGeneralOptionsHandler = s2.DisableGeneralOptionsHandler
	s.ErrorLog = s2.ErrorLog
	s.TLSNextProto = s2.TLSNextProto
}
