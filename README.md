# httprouter

The `josestg/httprouter` is a wrapper for the [julienschmidt/httprouter](github.com/julienschmidt/httprouter) package that modifies the handler signature to return an error and accept optional middleware.

## Installation

```bash
go get github.com/josestg/httprouter
```

## Usage

### Default Configuration

```go
func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))

	mux := httprouter.NewServeMux() // default configuration.
	/*
	   the default configuration:

	   mux := httprouter.NewServeMux(
	          httprouter.Options.HandleOption(true),
	          httprouter.Options.RedirectFixedPath(true),
	          httprouter.Options.RedirectTrailingSlash(true),
	          httprouter.Options.HandleMethodNotAllowed(true),
	          httprouter.Options.PanicHandler(httprouter.DefaultHandlers.Panic),
	          httprouter.Options.NotFoundHandler(httprouter.DefaultHandlers.NotFound()),
	          httprouter.Options.LastResortErrorHandler(httprouter.DefaultHandlers.LastResortError),
	          httprouter.Options.MethodNotAllowedHandler(httprouter.DefaultHandlers.MethodNotAllowed()),
	      )
	*/

	// register using httprouter.Handler
	mux.Handle("GET", "/", httprouter.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		_, err := fmt.Fprintf(w, "method: %s, url: %s", r.Method, r.URL)
		return err
	}))

	// or using httprouter.HandlerFunc directly.
	mux.HandleFunc("GET", "/ping", func(w http.ResponseWriter, r *http.Request) error {
		_, err := io.WriteString(w, "PONG!")
		return err
	})

	// or using httprouter.Route
	mux.Route(httprouter.Route{
		Method: "GET",
		Path:   "/hello",
		Handler: func(w http.ResponseWriter, r *http.Request) error {
			_, err := io.WriteString(w, "World!")
			return err
		},
	})

	log.Info("server is started")
	if err := http.ListenAndServe(":8081", mux); err != nil {
		log.Error("listen and serve failed", "error", err)
	}
}
```

### Global Middleware

```go
package main

import (
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/josestg/httprouter"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))

	global := httprouter.FoldMiddleware(
		// anything up here will be executed before the logged middleware.
		logged(log),
		// anything down here will be executed after the logged middleware.
	)

	mux := httprouter.NewServeMux(
		httprouter.Options.Middleware(global),
	)

	// or using httprouter.Route
	mux.Route(httprouter.Route{
		Method: "GET",
		Path:   "/hello",
		Handler: func(w http.ResponseWriter, r *http.Request) error {
			_, err := io.WriteString(w, "World!")
			return err
		},
	})

	log.Info("server is started")
	if err := http.ListenAndServe(":8081", mux); err != nil {
		log.Error("listen and serve failed", "error", err)
	}
}

func logged(log *slog.Logger) httprouter.Middleware {
	return func(h httprouter.Handler) httprouter.Handler {
		return httprouter.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			l := log.With("method", r.Method, "url", r.URL)
			if err := h.ServeHTTP(w, r); err != nil {
				l.ErrorContext(r.Context(), "request failed", "error", err)
			} else {
				l.InfoContext(r.Context(), "request succeeded")
			}
			return nil
		})
	}
}
```

### Route-Specific Middleware

```go
package main

import (
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/josestg/httprouter"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))

	mux := httprouter.NewServeMux()

	// or using httprouter.Route
	route := httprouter.Route{
		Method: "GET",
		Path:   "/hello",
		Handler: func(w http.ResponseWriter, r *http.Request) error {
			_, err := io.WriteString(w, "World!")
			return err
		},
	}

	// route-specific middleware, only applied to `GET /hello` route.
	mux.Route(route, logged(log))

	log.Info("server is started")
	if err := http.ListenAndServe(":8081", mux); err != nil {
		log.Error("listen and serve failed", "error", err)
	}
}

func logged(log *slog.Logger) httprouter.Middleware {
	return func(h httprouter.Handler) httprouter.Handler {
		return httprouter.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			l := log.With("method", r.Method, "url", r.URL)
			if err := h.ServeHTTP(w, r); err != nil {
				l.ErrorContext(r.Context(), "request failed", "error", err)
			} else {
				l.InfoContext(r.Context(), "request succeeded")
			}
			return nil
		})
	}
}
```

### Best Practice for using `mux.Route` instead of `mux.Handle`

Keep the route's Swagger docs and route configuration closed to minimize misconfigurations when there are changes to the API contract.

For example:

```go
package main

import (
    "encoding/json"
    "log/slog"
    "net/http"
    "os"

    "github.com/josestg/httprouter"
)

func main() {
    log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))

    mux := httprouter.NewServeMux()
    mux.Route(TodoRoute())

    log.Info("server is started")
    if err := http.ListenAndServe(":8081", mux); err != nil {
        log.Error("listen and serve failed", "error", err)
    }
}

type Todo struct {
    ID        int64  `json:"id"`
    Title     string `json:"title"`
    Completed bool   `json:"completed"`
}

// TodoRoute creates a new Todos route.
//
//	@Summary		Get list of todos.
//	@Accept			json
//	@Produce		json
//	@Success		200				{object}	[]Todo
//	@Router			/api/v1/todos [get]
func TodoRoute() httprouter.Route {
    return httprouter.Route{
        Method: "GET",
        Path:   "/api/v1/todos",
        Handler: func(w http.ResponseWriter, r *http.Request) error {
            w.WriteHeader(200)
            return json.NewEncoder(w).Encode([]Todo{{1, "todo 1", true}})
        },
    }
}
```



