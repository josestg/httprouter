package httprouter

import (
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// Middleware wraps the Handler with additional logic.
type Middleware func(Handler) Handler

// Then chains the middleware with the handler.
func (m Middleware) Then(h Handler) Handler { return m(h) }

// FoldMiddleware folds set of middlewares into a single middleware.
// For example:
//
//	FoldMiddleware(m1, m2, m3).Then(h)
//	will be equivalent to:
//	m1(m2(m3(h)))
func FoldMiddleware(middlewares ...Middleware) Middleware {
	return foldMiddlewares(middlewares)
}

func foldMiddlewares(middlewares []Middleware) Middleware {
	return func(next Handler) Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

type (
	// Param is alias of httprouter.Param.
	Param = httprouter.Param

	// Params is alias of httprouter.Params.
	Params = httprouter.Params
)

// PathParams gets the path variables from the request.
func PathParams(r *http.Request) Params {
	return httprouter.ParamsFromContext(r.Context())
}

// Handler is modified version of http.Handler.
type Handler interface {
	// ServeHTTP is just like http.Handler.ServeHTTP, but it returns an error.
	ServeHTTP(http.ResponseWriter, *http.Request) error
}

// HandlerFunc is a function that implements Handler.
// It is used to create a Handler from an ordinary function.
type HandlerFunc func(http.ResponseWriter, *http.Request) error

// ServeHTTP implements Handler.
func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) error { return f(w, r) }

// LastResortErrorHandler is the error handler that is called if after all middlewares,
// there is still an error occurs.
type LastResortErrorHandler func(http.ResponseWriter, *http.Request, error)

// Route is used to register a new handler to the ServeMux.
type Route struct {
	Method  string
	Path    string
	Handler HandlerFunc
}

// ServeMux is a wrapper of httprouter.Router with modified Handler.
// Instead of http.Handler, it uses Handler, which returns an error. This modification is used to simplify logic for
// creating a centralized error handler and logging.
//
// The ServeMux also supports MuxMiddleware, which is a middleware that wraps the Handler for all routes. Since the
// ServeMux also implements http.Handler, the NetMiddleware can be used to create middleware that will be executed
// before the ServeMux middleware.
//
// The ServeMux only exposes 3 methods: Route, Handle, and ServeHTTP, which are more simple than the original.
type ServeMux struct {
	core *httprouter.Router
	conf *Config
	midl Middleware

	// lastResortErrorHandler is the error handler that is called if after all middlewares,
	// there is still an error occurs. This handler is used to catch errors that are not handled by the middlewares.
	//
	// This handler is not part of the httprouter.Router, it is used by the ServeMux.
	lastResortErrorHandler LastResortErrorHandler
}

// NewServeMux creates a new ServeMux with given options.
// If no option is given, the Default option is applied.
func NewServeMux(opts ...Option) *ServeMux {
	mux := ServeMux{conf: &Config{
		RedirectTrailingSlash:  true,
		RedirectFixedPath:      true,
		HandleMethodNotAllowed: true,
		HandleOPTIONS:          true,
	}}

	for _, opt := range opts {
		opt(&mux)
	}
	// always apply the default options at the end.
	Options.Default()(&mux)

	mux.core = &httprouter.Router{
		RedirectTrailingSlash:  mux.conf.RedirectTrailingSlash,
		RedirectFixedPath:      mux.conf.RedirectFixedPath,
		HandleMethodNotAllowed: mux.conf.HandleMethodNotAllowed,
		HandleOPTIONS:          mux.conf.HandleOPTIONS,
		GlobalOPTIONS:          mux.conf.GlobalOPTIONS,
		NotFound:               mux.conf.NotFound,
		MethodNotAllowed:       mux.conf.MethodNotAllowed,
		PanicHandler:           mux.conf.PanicHandler,
	}
	return &mux
}

// Route is a syntactic sugar for Handle(method, path, handler) by using Route struct.
// This route also accepts variadic Middleware, which is applied to the route handler.
func (mux *ServeMux) Route(r Route, mid ...Middleware) {
	chain := foldMiddlewares(mid)
	mux.HandleFunc(r.Method, r.Path, chain.Then(r.Handler).ServeHTTP)
}

// HandleFunc just like Handle, but it accepts HandlerFunc.
func (mux *ServeMux) HandleFunc(method, path string, handler HandlerFunc) {
	mux.Handle(method, path, handler)
}

// Handle registers a new request handler with the given method and path.
func (mux *ServeMux) Handle(method, path string, handler Handler) {
	mux.core.HandlerFunc(method, path, func(w http.ResponseWriter, r *http.Request) {
		err := mux.midl.Then(handler).ServeHTTP(w, r)
		if err != nil {
			mux.lastResortErrorHandler(w, r, err)
		}
	})
}

// ServeHTTP satisfies http.Handler.
func (mux *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) { mux.core.ServeHTTP(w, r) }

// Config is the configuration for the underlying httprouter.Router.
type Config struct {
	// Enables automatic redirection if the current route can't be matched but a
	// handler for the path with (without) the trailing slash exists.
	// For example if /foo/ is requested but a route only exists for /foo, the
	// client is redirected to /foo with http status code 301 for GET requests
	// and 307 for all other request methods.
	RedirectTrailingSlash bool

	// If enabled, the router tries to fix the current request path, if no
	// handle is registered for it.
	// First superfluous path elements like ../ or // are removed.
	// Afterward the router does a case-insensitive lookup of the cleaned path.
	// If a handle can be found for this route, the router makes a redirection
	// to the corrected path with status code 301 for GET requests and 307 for
	// all other request methods.
	// For example /FOO and /..//Foo could be redirected to /foo.
	// RedirectTrailingSlash is independent of this option.
	RedirectFixedPath bool

	// If enabled, the router checks if another method is allowed for the
	// current route, if the current request can not be routed.
	// If this is the case, the request is answered with 'Method Not Allowed'
	// and HTTP status code 405.
	// If no other Method is allowed, the request is delegated to the NotFound
	// handler.
	HandleMethodNotAllowed bool

	// If enabled, the router automatically replies to OPTIONS requests.
	// Custom OPTIONS handlers take priority over automatic replies.
	HandleOPTIONS bool

	// An optional http.Handler that is called on automatic OPTIONS requests.
	// The handler is only called if HandleOPTIONS is true and no OPTIONS
	// handler for the specific path was set.
	// The "Allowed" header is set before calling the handler.
	GlobalOPTIONS http.Handler

	// Configurable http.Handler which is called when no matching route is
	// found.
	NotFound http.Handler

	// Configurable http.Handler which is called when a request
	// cannot be routed and HandleMethodNotAllowed is true.
	// The "Allow" header with allowed request methods is set before the handler
	// is called.
	MethodNotAllowed http.Handler

	// Function to handle panics recovered from http handlers.
	// It should be used to generate an error page and return the http error code
	// 500 (Internal Server Error).
	// The handler can be used to keep your server from crashing because of
	// unrecoverable panics.
	PanicHandler func(http.ResponseWriter, *http.Request, any)
}

// nsOpts is an internal type for grouping options.
type nsOpts int

// Option is a function that configures the ServeMux.
type Option func(*ServeMux)

func applyOptions(mux *ServeMux, opts []Option) {
	for _, opt := range opts {
		opt(mux)
	}
}

// Options is a namespace for accessing options.
const Options nsOpts = 0

// Default configures the ServeMux with default options.
func (nsOpts) Default() Option {
	return func(mux *ServeMux) {
		defaults := make([]Option, 0, 5) // at most 5 default options.
		if mux.lastResortErrorHandler == nil {
			defaults = append(defaults, Options.LastResortErrorHandler(DefaultHandlers.LastResortError))
		}

		if mux.conf.NotFound == nil {
			defaults = append(defaults, Options.NotFoundHandler(DefaultHandlers.NotFound()))
		}

		if mux.conf.MethodNotAllowed == nil {
			defaults = append(defaults, Options.MethodNotAllowedHandler(DefaultHandlers.MethodNotAllowed()))
		}

		if mux.conf.PanicHandler == nil {
			defaults = append(defaults, Options.PanicHandler(DefaultHandlers.Panic))
		}

		if mux.midl == nil {
			// add an identity middleware, to avoid nil pointer dereference check.
			defaults = append(defaults, Options.Middleware(func(h Handler) Handler { return h }))
		}
		applyOptions(mux, defaults)
	}
}

// RedirectTrailingSlash enables/disables automatic redirection if the current route can't be matched but a
// handler for the path with (without) the trailing slash exists. Default enabled.
//
// see: https://godoc.org/github.com/julienschmidt/httprouter#Router.RedirectTrailingSlash
func (nsOpts) RedirectTrailingSlash(enabled bool) Option {
	return func(mux *ServeMux) { mux.conf.RedirectTrailingSlash = enabled }
}

// RedirectFixedPath if enabled, the router tries to fix the current request path, if no
// handle is registered for it. Default enabled.
//
// see: https://godoc.org/github.com/julienschmidt/httprouter#Router.RedirectFixedPath
func (nsOpts) RedirectFixedPath(enabled bool) Option {
	return func(mux *ServeMux) { mux.conf.RedirectFixedPath = enabled }
}

// HandleMethodNotAllowed if enabled, the router checks if another method is allowed for the
// current route, if the current request can not be routed. Default enabled.
//
// see: https://godoc.org/github.com/julienschmidt/httprouter#Router.HandleMethodNotAllowed
func (nsOpts) HandleMethodNotAllowed(enabled bool) Option {
	return func(mux *ServeMux) { mux.conf.HandleMethodNotAllowed = enabled }
}

// HandleOption if enabled, the router automatically replies to OPTIONS requests.
// Custom OPTIONS handlers take priority over automatic replies. Default enabled.
//
// see: https://godoc.org/github.com/julienschmidt/httprouter#Router.HandleOPTIONS
func (nsOpts) HandleOption(enabled bool) Option {
	return func(mux *ServeMux) { mux.conf.HandleOPTIONS = enabled }
}

// GlobalOptionHandler sets the global OPTIONS handler.
// The handler is only called if HandleOPTIONS is true and no OPTIONS handler for the specific path was set.
//
// see: https://godoc.org/github.com/julienschmidt/httprouter#Router.GlobalOPTIONS
func (nsOpts) GlobalOptionHandler(handler http.Handler) Option {
	return func(mux *ServeMux) { mux.conf.GlobalOPTIONS = handler }
}

// NotFoundHandler sets the handler that is called when no matching route is found.
// If it is not set, DefaultHandlers.NotFound is used.
func (nsOpts) NotFoundHandler(handler http.Handler) Option {
	return func(mux *ServeMux) { mux.conf.NotFound = handler }
}

// MethodNotAllowedHandler sets the handler that is called when a request
// cannot be routed and HandleMethodNotAllowed is true. If it is not set, DefaultHandlers.MethodNotAllowed is used.
func (nsOpts) MethodNotAllowedHandler(handler http.Handler) Option {
	return func(mux *ServeMux) { mux.conf.MethodNotAllowed = handler }
}

// PanicHandler sets the handler that is called when a panic occurs.
// If no handler is set, the DefaultHandlers.LastResortError is used.
func (nsOpts) PanicHandler(handler func(http.ResponseWriter, *http.Request, any)) Option {
	return func(mux *ServeMux) { mux.conf.PanicHandler = handler }
}

// LastResortErrorHandler sets the handler that is called if after all middlewares,
// there is still an error occurs.
// This handler is used to catch errors that are not handled by the middlewares.
func (nsOpts) LastResortErrorHandler(handler LastResortErrorHandler) Option {
	return func(mux *ServeMux) { mux.lastResortErrorHandler = handler }
}

// Middleware sets the middleware for all routes in the ServeMux.
// This middleware is called before the request is received by the Route Handler, that means if route has specific
// middleware, it will be called after this middleware. In other words, this middleware is the outermost middleware.
func (nsOpts) Middleware(m Middleware) Option {
	return func(mux *ServeMux) { mux.midl = m }
}

// nsDefaultHandlers is an internal type for grouping default handlers.
type nsDefaultHandlers int

// DefaultHandlers is a namespace for accessing default handlers.
const DefaultHandlers nsDefaultHandlers = 0

// LastResortError is the default last resort error handler.
func (nsDefaultHandlers) LastResortError(w http.ResponseWriter, r *http.Request, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = fmt.Fprintf(w, "default last resort error handler: method: %s, path: %s, error: %v", r.Method, r.URL.Path, err)
}

// NotFound is the default not found handler.
func (nsDefaultHandlers) NotFound() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "default not found handler: method: %s, path: %s", r.Method, r.URL.Path)
	}
}

// MethodNotAllowed is the default method not allowed handler.
func (nsDefaultHandlers) MethodNotAllowed() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = fmt.Fprintf(w, "default method not allowed handler: method: %s, path: %s", r.Method, r.URL.Path)
	}
}

// Panic is the default panic handler.
func (nsDefaultHandlers) Panic(w http.ResponseWriter, r *http.Request, v any) {
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = fmt.Fprintf(w, "default panic handler: method: %s, path: %s, error: %v", r.Method, r.URL.Path, v)
}
