package grit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// ========================
// Supabase Client Config
// ========================

var (
	supabaseURL string
	supabaseKey string
)

// SupabaseInitClient stores the Supabase project URL and API key.
//
// supabaseURL  — e.g. "https://xyzcompany.supabase.co"
// supabaseKey  — your anon/service_role key from the Supabase dashboard
func SupabaseInitClient(url, key string) {
	supabaseURL = url
	supabaseKey = key
	fmt.Println("✅ Supabase initialized:", url)
}

// ========================
// Internal HTTP Helper
// ========================

func supabaseRequest(method, path string, body interface{}) (*http.Response, error) {
	if supabaseURL == "" || supabaseKey == "" {
		return nil, fmt.Errorf("supabase not initialized, call SupabaseInit() first")
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, supabaseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", supabaseKey)
	req.Header.Set("Authorization", "Bearer "+supabaseKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	return http.DefaultClient.Do(req)
}

// ========================
// Internal Model Helpers
// ========================

// supabaseBodyToMap decodes the HTTP request body.
// If a model is registered for 'name', the body is decoded into that model
// struct first (for field validation / filtering), then bridged to a raw map
// before forwarding to the Supabase REST API.
// Falls back to raw map[string]interface{} if no model is registered.
func supabaseBodyToMap(r *http.Request, name string) (map[string]interface{}, error) {
	model := models[name]
	if model != nil {
		obj := clone(model)
		if err := json.NewDecoder(r.Body).Decode(obj); err != nil {
			return nil, err
		}
		b, _ := json.Marshal(obj)
		var m map[string]interface{}
		_ = json.Unmarshal(b, &m)
		return m, nil
	}
	var m map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// supabaseDecodeResponse decodes a Supabase REST response JSON body.
// If a model is registered for 'name', the result is decoded into a typed
// slice ([]Model) or single model instance.
// Otherwise returns a raw interface{}.
func supabaseDecodeResponse(body io.Reader, name string) interface{} {
	model := models[name]
	if model != nil {
		slice := makeSlice(model)
		_ = json.NewDecoder(body).Decode(slice)
		return slice
	}
	var result interface{}
	_ = json.NewDecoder(body).Decode(&result)
	return result
}

// supabaseDecodeSingleResponse decodes a Supabase response that returns an
// array but we need only the first element (e.g. GetByID).
// If a model is registered, decodes into the model struct; otherwise raw map.
func supabaseDecodeSingleResponse(body io.Reader, name string) (interface{}, bool) {
	// First decode as raw array regardless — Supabase always returns an array
	var rawArr []json.RawMessage
	if err := json.NewDecoder(body).Decode(&rawArr); err != nil || len(rawArr) == 0 {
		return nil, false
	}

	model := models[name]
	if model != nil {
		obj := clone(model)
		if err := json.Unmarshal(rawArr[0], obj); err != nil {
			return nil, false
		}
		return obj, true
	}

	var raw map[string]interface{}
	_ = json.Unmarshal(rawArr[0], &raw)
	return raw, true
}

// ========================
// SIGNUP — Supabase Auth
// ========================

// SupabaseSignupHandler signs up a new user via Supabase Auth REST API,
// then issues a signed app-level JWT.
func SupabaseSignupHandler(jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			methodNotAllowed(w, r, "POST")
			return
		}

		if supabaseURL == "" {
			respond(w, 500, false, "Supabase not initialized. Call grit.SupabaseInit() first.", nil)
			return
		}

		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respond(w, 400, false, "Invalid request body", nil)
			return
		}

		if body.Email == "" || body.Password == "" {
			respond(w, 400, false, "Email and password are required", nil)
			return
		}

		if len(body.Password) < 6 {
			respond(w, 400, false, "Password must be >= 6 characters", nil)
			return
		}

		resp, err := supabaseRequest(http.MethodPost, "/auth/v1/signup", map[string]string{
			"email":    body.Email,
			"password": body.Password,
		})
		if err != nil {
			respond(w, 500, false, "Supabase request failed: "+err.Error(), nil)
			return
		}
		defer resp.Body.Close()

		var sbResponse map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&sbResponse)

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			msg := "Signup failed"
			if m, ok := sbResponse["msg"].(string); ok {
				msg = m
			} else if m, ok := sbResponse["message"].(string); ok {
				msg = m
			}
			respond(w, resp.StatusCode, false, msg, nil)
			return
		}

		userID, _ := sbResponse["id"].(string)
		email, _ := sbResponse["email"].(string)

		token := buildSupabaseJWT(userID, email, jwtSecret)

		respond(w, 201, true, "Signup successful", map[string]interface{}{
			"token": token,
			"user": map[string]interface{}{
				"id":    userID,
				"email": email,
			},
		})
	}
}

// ========================
// SIGNIN — Supabase Auth
// ========================

// SupabaseSigninHandler signs in a user via Supabase Auth REST API,
// then issues a signed app-level JWT.
func SupabaseSigninHandler(jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			methodNotAllowed(w, r, "POST")
			return
		}

		if supabaseURL == "" {
			respond(w, 500, false, "Supabase not initialized. Call grit.SupabaseInit() first.", nil)
			return
		}

		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respond(w, 400, false, "Invalid request body", nil)
			return
		}

		resp, err := supabaseRequest(http.MethodPost, "/auth/v1/token?grant_type=password", map[string]string{
			"email":    body.Email,
			"password": body.Password,
		})
		if err != nil {
			respond(w, 500, false, "Supabase request failed: "+err.Error(), nil)
			return
		}
		defer resp.Body.Close()

		var sbResponse map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&sbResponse)

		if resp.StatusCode != http.StatusOK {
			msg := "Invalid credentials"
			if m, ok := sbResponse["error_description"].(string); ok {
				msg = m
			}
			respond(w, 401, false, msg, nil)
			return
		}

		// sbResponse contains Supabase user object nested inside "user"
		userMap, _ := sbResponse["user"].(map[string]interface{})
		userID, _ := userMap["id"].(string)
		email, _ := userMap["email"].(string)

		token := buildSupabaseJWT(userID, email, jwtSecret)

		respond(w, 200, true, "Signin successful", map[string]interface{}{
			"token": token,
			"user": map[string]interface{}{
				"id":    userID,
				"email": email,
			},
		})
	}
}

// ========================
// CREATE — POST /rest/v1/{table}
// ========================

// SupabaseC creates a new record in a Supabase table.
//
// If a model is registered for 'name' via grit.RegisterModel(), the
// request body is decoded into that model for field validation before
// forwarding to Supabase.
//
// Usage (with model):
//
//	grit.RegisterModel("posts", &Post{})
//	r.Post("/posts", grit.SupabaseC("posts"))
//
// Usage (without model — raw map):
//
//	r.Post("/posts", grit.SupabaseC("posts"))
func SupabaseC(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			methodNotAllowed(w, r, "POST")
			return
		}

		body, err := supabaseBodyToMap(r, name)
		if err != nil {
			respond(w, 400, false, "Invalid request body", nil)
			return
		}

		resp, err := supabaseRequest(http.MethodPost, "/rest/v1/"+name, body)
		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			var result interface{}
			json.NewDecoder(resp.Body).Decode(&result)
			respond(w, resp.StatusCode, false, "Supabase error", result)
			return
		}

		result := supabaseDecodeResponse(resp.Body, name)
		respond(w, 201, true, "Created successfully", result)
	}
}

// ========================
// READ ALL — GET /rest/v1/{table}
// ========================

// SupabaseR fetches all records from a Supabase table.
// Supports optional query params forwarded to Supabase (e.g. ?select=*, ?order=created_at.desc).
//
// If a model is registered for 'name', results are decoded into a typed slice.
//
// Usage:
//
//	grit.RegisterModel("posts", &Post{})
//	r.Get("/posts", grit.SupabaseR("posts"))
func SupabaseR(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodGet {
			methodNotAllowed(w, r, "GET")
			return
		}

		// Forward any query params (filter, select, order, limit)
		queryString := ""
		if q := r.URL.RawQuery; q != "" {
			queryString = "?" + q
		}

		resp, err := supabaseRequest(http.MethodGet, "/rest/v1/"+name+queryString, nil)
		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			var result interface{}
			json.NewDecoder(resp.Body).Decode(&result)
			respond(w, resp.StatusCode, false, "Supabase error", result)
			return
		}

		result := supabaseDecodeResponse(resp.Body, name)
		respond(w, 200, true, "Fetched successfully", result)
	}
}

// ========================
// READ BY ID — GET /rest/v1/{table}?id=eq.{id}
// ========================

// SupabaseGetByID fetches a single record from a Supabase table by its `id` query param.
// If a model is registered for 'name', the result is decoded into that model struct.
//
// Usage:
//
//	grit.RegisterModel("posts", &Post{})
//	r.Get("/post", grit.SupabaseGetByID("posts"))
//
// Query:  GET /post?id=123
func SupabaseGetByID(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodGet {
			methodNotAllowed(w, r, "GET")
			return
		}

		id := r.URL.Query().Get("id")
		if id == "" {
			respond(w, 400, false, "id query parameter is required", nil)
			return
		}

		resp, err := supabaseRequest(http.MethodGet, "/rest/v1/"+name+"?id=eq."+id+"&limit=1", nil)
		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			var result interface{}
			json.NewDecoder(resp.Body).Decode(&result)
			respond(w, resp.StatusCode, false, "Supabase error", result)
			return
		}

		record, found := supabaseDecodeSingleResponse(resp.Body, name)
		if !found {
			respond(w, 404, false, "Record not found", nil)
			return
		}

		respond(w, 200, true, "Fetched successfully", record)
	}
}

// ========================
// UPDATE — PATCH /rest/v1/{table}?id=eq.{id}
// ========================

// SupabaseU updates a record in a Supabase table. Requires `id` in the JSON body.
// If a model is registered for 'name', the update payload is validated through the model.
//
// Usage:
//
//	grit.RegisterModel("posts", &Post{})
//	r.Put("/post", grit.SupabaseU("posts"))
//
// Body:   { "id": 1, "title": "Updated Title" }
func SupabaseU(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPut && r.Method != http.MethodPatch {
			methodNotAllowed(w, r, "PUT or PATCH")
			return
		}

		// Always decode as raw map first so we can safely extract "id"
		var rawPayload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&rawPayload); err != nil {
			respond(w, 400, false, "Invalid request body", nil)
			return
		}

		idRaw, ok := rawPayload["id"]
		if !ok {
			respond(w, 400, false, "id field is required in body", nil)
			return
		}

		id := fmt.Sprintf("%v", idRaw)
		delete(rawPayload, "id") // never send id in the update payload

		if len(rawPayload) == 0 {
			respond(w, 400, false, "No fields to update", nil)
			return
		}

		// If model registered, filter payload through it to validate fields
		model := models[name]
		if model != nil {
			b, _ := json.Marshal(rawPayload)
			obj := clone(model)
			_ = json.Unmarshal(b, obj)
			b2, _ := json.Marshal(obj)
			var filtered map[string]interface{}
			_ = json.Unmarshal(b2, &filtered)
			rawPayload = filtered
		}

		resp, err := supabaseRequest(http.MethodPatch, "/rest/v1/"+name+"?id=eq."+id, rawPayload)
		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			var result interface{}
			json.NewDecoder(resp.Body).Decode(&result)
			respond(w, resp.StatusCode, false, "Supabase error", result)
			return
		}

		result := supabaseDecodeResponse(resp.Body, name)
		respond(w, 200, true, "Updated successfully", result)
	}
}

// ========================
// DELETE — DELETE /rest/v1/{table}?id=eq.{id}
// ========================

// SupabaseD deletes a record from a Supabase table. Requires `id` in the JSON body.
//
// Usage:
//
//	r.Delete("/post", grit.SupabaseD("posts"))
//
// Body:   { "id": 1 }
func SupabaseD(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodDelete {
			methodNotAllowed(w, r, "DELETE")
			return
		}

		var body struct {
			ID interface{} `json:"id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == nil {
			respond(w, 400, false, "id is required in body", nil)
			return
		}

		id := fmt.Sprintf("%v", body.ID)

		resp, err := supabaseRequest(http.MethodDelete, "/rest/v1/"+name+"?id=eq."+id, nil)
		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			var result interface{}
			json.NewDecoder(resp.Body).Decode(&result)
			respond(w, resp.StatusCode, false, "Supabase error", result)
			return
		}

		respond(w, 200, true, "Deleted successfully", nil)
	}
}

// ========================
// JWT Builder (Supabase-specific)
// ========================

func buildSupabaseJWT(userID, email, secret string) string {
	claims := jwt.MapClaims{
		"sub":         userID,
		"email":       email,
		"permissions": []string{"user:read"},
		"exp":         time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(secret))
	return signed
}
