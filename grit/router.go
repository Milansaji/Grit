package grit

import (
	"log"
	"net/http"
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
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {

		if methodRoutes, ok := r.routes[req.Method]; ok {
			if handler, exists := methodRoutes[req.URL.Path]; exists {
				handler(w, req)
				return
			}
		}

		// fallback: route not found
		HandleNotFound(w, req)
	})

	log.Printf("Server starting on http://localhost:%s/", port)
	return http.ListenAndServe(":"+port, nil)
}

// internal route handler
func (r *Router) handle(method, path string, handler http.HandlerFunc) {
	if r.routes[method] == nil {
		r.routes[method] = make(map[string]http.HandlerFunc)
	}
	r.routes[method][path] = handler
	log.Printf("[%s] route registered: %s", method, path)
}

// Get registers a GET route
func (r *Router) Get(path string, handlerFunc http.HandlerFunc) {
	r.handle("GET", path, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		handlerFunc(w, req)
		log.Printf("[GET] request for %s handled successfully", path)
	})
}

// Post registers a POST route
func (r *Router) Post(path string, handlerFunc http.HandlerFunc) {
	r.handle("POST", path, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		handlerFunc(w, req)
		log.Printf("[POST] request for %s handled successfully", path)
	})
}

// Put registers a PUT route
func (r *Router) Put(path string, handlerFunc http.HandlerFunc) {
	r.handle("PUT", path, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPut {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		handlerFunc(w, req)
		log.Printf("[PUT] request for %s handled successfully", path)
	})
}

// Delete registers a DELETE route
func (r *Router) Delete(path string, handlerFunc http.HandlerFunc) {
	r.handle("DELETE", path, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		handlerFunc(w, req)
		log.Printf("[DELETE] request for %s handled successfully", path)
	})
}
