package grit

import (
	"encoding/json"
	"net/http"
	"time"
)

// ---------- response helpers ----------

type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

type Meta struct {
	Timestamp string `json:"timestamp"`
}

func respond(w http.ResponseWriter, status int, success bool, msg string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	res := APIResponse{
		Success: success,
		Message: msg,
		Data:    data,
		Meta: &Meta{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}

	_ = json.NewEncoder(w).Encode(res)
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request, allowed string) {
	respond(
		w,
		http.StatusMethodNotAllowed,
		false,
		"Method "+r.Method+" not allowed. Use "+allowed,
		nil,
	)
}
