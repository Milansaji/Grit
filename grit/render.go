package grit

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"text/template"
)

// RenderTemplate renders HTML templates from /templates directory
func RenderTemplate(w http.ResponseWriter, filename string) {

	// secure path handling
	path := filepath.Join("templates", filepath.Clean(filename))

	// prevent directory traversal
	if !strings.HasPrefix(path, "templates/") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	tpl, err := template.ParseFiles(path)
	if err != nil {
		log.Printf("Template parsing error for %s: %v", path, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := tpl.Execute(w, nil); err != nil {
		log.Printf("Template execution error for %s: %v", path, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// HandleNotFound handles 404 routes
func HandleNotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprint(
		w,
		"<h1>404 - Page Not Found</h1><p>The page you're looking for doesn't exist.</p>",
	)

	log.Printf("404 Not Found: [%s] %s", r.Method, r.URL.Path)
}

// RenderText writes plain text response
func RenderText(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write([]byte(text)); err != nil {
		log.Printf("Error writing text response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// RenderJSON writes JSON response
func RenderJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshalling JSON: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if _, err := w.Write(jsonData); err != nil {
		log.Printf("Error writing JSON response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
