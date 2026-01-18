package grit

import (
	"log"
	"net/http"
	"time"
)

// ----------------------
// ANSI COLORS
// ----------------------
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Gray   = "\033[90m"
)

// Router is a simple method-based router
type Router struct {
	routes map[string]map[string]http.HandlerFunc
}

// New creates a new Router
func New() *Router {
	return &Router{
		routes: make(map[string]map[string]http.HandlerFunc),
	}
}

// Start starts the HTTP server
func (r *Router) Start(port string) error {

	routerHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

		// Handle OPTIONS
		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if methodRoutes, ok := r.routes[req.Method]; ok {
			if handler, exists := methodRoutes[req.URL.Path]; exists {
				handler(w, req)
				return
			}
		}

		log.Printf("%s[404]%s %s %s",
			Red, Reset,
			req.Method,
			req.URL.Path,
		)

		HandleNotFound(w, req)
	})

	handler := loggingMiddleware(corsMiddleware(routerHandler))

	log.Printf("%s🚀 Server running at http://localhost:%s%s", Green, port, Reset)
	http.Handle("/", handler)

	return http.ListenAndServe(":"+port, nil)
}

// ----------------------
// LOGGING MIDDLEWARE
// ----------------------
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)

		color := methodColor(r.Method)

		log.Printf(
			"%s[%s]%s %s %s %s(%v)%s",
			color,
			r.Method,
			Reset,
			r.RemoteAddr,
			r.URL.Path,
			Gray,
			duration,
			Reset,
		)
	})
}

// ----------------------
// CORS MIDDLEWARE
// ----------------------
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ----------------------
// ROUTE REGISTRATION
// ----------------------
func (r *Router) handle(method, path string, handler http.HandlerFunc) {
	if r.routes[method] == nil {
		r.routes[method] = make(map[string]http.HandlerFunc)
	}
	r.routes[method][path] = handler

	log.Printf(
		"%s🧩 [%s]%s route registered: %s",
		Blue,
		method,
		Reset,
		path,
	)
}

func (r *Router) Get(path string, handler http.HandlerFunc) {
	r.handle(http.MethodGet, path, handler)
}

func (r *Router) Post(path string, handler http.HandlerFunc) {
	r.handle(http.MethodPost, path, handler)
}

func (r *Router) Put(path string, handler http.HandlerFunc) {
	r.handle(http.MethodPut, path, handler)
}

func (r *Router) Delete(path string, handler http.HandlerFunc) {
	r.handle(http.MethodDelete, path, handler)
}

// ----------------------
// METHOD COLOR HELPER
// ----------------------
func methodColor(method string) string {
	switch method {
	case http.MethodGet:
		return Green
	case http.MethodPost:
		return Yellow
	case http.MethodPut:
		return Purple
	case http.MethodDelete:
		return Red
	default:
		return Gray
	}
}
