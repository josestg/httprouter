# httprouter

The `josestg/httprouterx` is a wrapper for the [julienschmidt/httprouter](https://github.com/julienschmidt/httprouter) package that modifies the handler signature to return an error and accept optional middleware.

## Installation

```bash
go get github.com/josestg/httprouterx
```

## Usage

### Default Configuration

```go
func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))

	mux := httprouterx.NewServeMux() // default configuration.
	/*
	   the default configuration:

	   mux := httprouterx.NewServeMux(
	          httprouterx.Options.HandleOption(true),
	          httprouterx.Options.RedirectFixedPath(true),
	          httprouterx.Options.RedirectTrailingSlash(true),
	          httprouterx.Options.HandleMethodNotAllowed(true),
	          httprouterx.Options.PanicHandler(httprouterx.DefaultHandlers.Panic),
	          httprouterx.Options.NotFoundHandler(httprouterx.DefaultHandlers.NotFound()),
	          httprouterx.Options.LastResortErrorHandler(httprouterx.DefaultHandlers.LastResortError),
	          httprouterx.Options.MethodNotAllowedHandler(httprouterx.DefaultHandlers.MethodNotAllowed()),
	      )
	*/

	// register using httprouterx.Handler
	mux.Handle("GET", "/", httprouterx.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		_, err := fmt.Fprintf(w, "method: %s, url: %s", r.Method, r.URL)
		return err
	}))

	// or using httprouterx.HandlerFunc directly.
	mux.HandleFunc("GET", "/ping", func(w http.ResponseWriter, r *http.Request) error {
		_, err := io.WriteString(w, "PONG!")
		return err
	})

	// or using httprouterx.Route
	mux.Route(httprouterx.Route{
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
func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))

	global := httprouterx.FoldMiddleware(
		// anything up here will be executed before the logged middleware.
		logged(log),
		// anything down here will be executed after the logged middleware.
	)

	mux := httprouterx.NewServeMux(
		httprouterx.Options.Middleware(global),
	)

	// or using httprouterx.Route
	mux.Route(httprouterx.Route{
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

func logged(log *slog.Logger) httprouterx.Middleware {
	return func(h httprouterx.Handler) httprouterx.Handler {
		return httprouterx.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
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
func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))

	mux := httprouterx.NewServeMux()

	// or using httprouterx.Route
	route := httprouterx.Route{
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

func logged(log *slog.Logger) httprouterx.Middleware {
	return func(h httprouterx.Handler) httprouterx.Handler {
		return httprouterx.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
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
func main() {
    log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))

    mux := httprouterx.NewServeMux()
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
func TodoRoute() httprouterx.Route {
    return httprouterx.Route{
        Method: "GET",
        Path:   "/api/v1/todos",
        Handler: func(w http.ResponseWriter, r *http.Request) error {
            w.WriteHeader(200)
            return json.NewEncoder(w).Encode([]Todo{{1, "todo 1", true}})
        },
    }
}
```



