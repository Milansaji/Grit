package grit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// OpenCollection opens a sqlite db per collection
func OpenCollection(name string) (*gorm.DB, interface{}, error) {

	model := models[name]
	if model == nil {
		return nil, nil, fmt.Errorf("model not registered: %s", name)
	}

	dbPath := fmt.Sprintf("%s.db", name)
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, nil, err
	}

	// auto create table
	if err := db.AutoMigrate(model); err != nil {
		return nil, nil, err
	}

	return db, model, nil
}

// ---------- CREATE ----------
func GritC(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			methodNotAllowed(w, r, "POST")
			return
		}

		db, model, err := OpenCollection(name)
		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}

		obj := clone(model)
		if err := json.NewDecoder(r.Body).Decode(obj); err != nil {
			respond(w, 400, false, "Invalid request body", nil)
			return
		}

		db.Create(obj)
		respond(w, 201, true, "Created successfully", obj)
	}
}

// ---------- READ ALL ----------
func GritR(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodGet {
			methodNotAllowed(w, r, "GET")
			return
		}

		db, model, err := OpenCollection(name)
		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}

		slice := makeSlice(model)
		db.Find(slice)

		respond(w, 200, true, "Fetched successfully", slice)
	}
}

// ---------- READ BY ID (BODY OR QUERY) ----------
func GritGetByID(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			methodNotAllowed(w, r, "POST")
			return
		}

		var id uint

		// try body
		var body struct {
			ID uint `json:"id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.ID != 0 {
			id = body.ID
		}

		// fallback query param
		if id == 0 {
			idStr := r.URL.Query().Get("id")
			if idStr != "" {
				parsed, _ := strconv.Atoi(idStr)
				id = uint(parsed)
			}
		}

		if id == 0 {
			respond(w, 400, false, "ID is required", nil)
			return
		}

		db, model, err := OpenCollection(name)
		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}

		obj := clone(model)
		result := db.First(obj, "id = ?", id)

		if result.RowsAffected == 0 {
			respond(w, 404, false, "Record not found", nil)
			return
		}

		respond(w, 200, true, "Fetched successfully", obj)
	}
}

// ---------- UPDATE ----------
func GritU(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPut {
			methodNotAllowed(w, r, "PUT")
			return
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			respond(w, 400, false, "Invalid request body", nil)
			return
		}

		idRaw, ok := payload["id"]
		if !ok {
			respond(w, 400, false, "ID required", nil)
			return
		}

		delete(payload, "id")

		db, model, err := OpenCollection(name)
		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}

		obj := clone(model)
		result := db.Model(obj).
			Where("id = ?", uint(idRaw.(float64))).
			Updates(payload)

		if result.RowsAffected == 0 {
			respond(w, 404, false, "Record not found", nil)
			return
		}

		respond(w, 200, true, "Updated successfully", nil)
	}
}

// ---------- DELETE ----------
func GritD(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodDelete {
			methodNotAllowed(w, r, "DELETE")
			return
		}

		var body struct {
			ID uint `json:"id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == 0 {
			respond(w, 400, false, "Invalid ID", nil)
			return
		}

		db, model, err := OpenCollection(name)
		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}

		obj := clone(model)
		result := db.Delete(obj, "id = ?", body.ID)

		if result.RowsAffected == 0 {
			respond(w, 404, false, "Record not found", nil)
			return
		}

		respond(w, 200, true, "Deleted successfully", nil)
	}
}
