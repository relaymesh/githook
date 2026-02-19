package server

import "net/http"

// Middleware wraps an http.Handler.
type Middleware func(http.Handler) http.Handler

func applyMiddlewares(handler http.Handler, middlewares []Middleware) http.Handler {
	if handler == nil {
		return nil
	}
	wrapped := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] == nil {
			continue
		}
		wrapped = middlewares[i](wrapped)
	}
	return wrapped
}
