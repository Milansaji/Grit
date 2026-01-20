package grit

import (
	"log"
	"net/http"
	"time"
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Gray   = "\033[90m"
)

type Router struct {
	routes map[string]map[string]http.HandlerFunc
}

func New() *Router {
	r := &Router{
		routes: make(map[string]map[string]http.HandlerFunc),
	}

	// Built-in docs
	r.handle(http.MethodGet, "/docs", DocsHandler())
	r.handle(http.MethodGet, "/openapi.json", OpenAPIHandler())

	log.Printf("%s📘 Swagger enabled at /docs%s", Blue, Reset)
	return r
}

func (r *Router) Start(port string) error {

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if m, ok := r.routes[req.Method]; ok {
			if h, ok := m[req.URL.Path]; ok {
				h(w, req)
				return
			}
		}

		log.Printf("%s[404]%s %s %s", Red, Reset, req.Method, req.URL.Path)
		HandleNotFound(w, req)
	})

	h := loggingMiddleware(corsMiddleware(handler))

	log.Printf("%s🚀 Server http://localhost:%s%s", Green, port, Reset)
	log.Printf("%s📘 Docs http://localhost:%s/docs%s", Blue, port, Reset)

	http.Handle("/", h)
	return http.ListenAndServe(":"+port, nil)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s[%s]%s %s %s %s(%v)%s",
			methodColor(r.Method), r.Method, Reset,
			r.RemoteAddr, r.URL.Path,
			Gray, time.Since(start), Reset,
		)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		if r.Method == http.MethodOptions {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (r *Router) handle(method, path string, h http.HandlerFunc) {
	if r.routes[method] == nil {
		r.routes[method] = map[string]http.HandlerFunc{}
	}
	r.routes[method][path] = h

	// auto register minimal doc
	registerDocs(method, path)
}

func (r *Router) Get(p string, h http.HandlerFunc)    { r.handle(http.MethodGet, p, h) }
func (r *Router) Post(p string, h http.HandlerFunc)   { r.handle(http.MethodPost, p, h) }
func (r *Router) Put(p string, h http.HandlerFunc)    { r.handle(http.MethodPut, p, h) }
func (r *Router) Delete(p string, h http.HandlerFunc) { r.handle(http.MethodDelete, p, h) }

func methodColor(m string) string {
	switch m {
	case "GET":
		return Green
	case "POST":
		return Yellow
	case "PUT":
		return Purple
	case "DELETE":
		return Red
	default:
		return Gray
	}
}
