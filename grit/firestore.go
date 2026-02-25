package grit

import (
	"context"
	"encoding/json"
	"net/http"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/option"
)

// ========================
// Firestore Client
// ========================

var firestoreClient *firestore.Client

// FirestoreInitClient initializes the Firestore database client.
//
// projectID  — your Firebase project ID (e.g. "my-app-12345").
// credPath   — optional path to serviceAccountKey.json.
//
//	Pass it when running locally without Application Default
//	Credentials configured (i.e. outside GCP / Cloud Run).
//	When deployed on GCP, omit it and ADC is used automatically.
//
// Example (local):
//
//	grit.FirestoreInit(projectID, "serviceAccountKey.json")
//
// Example (GCP / Cloud Run):
//
//	grit.FirestoreInit(projectID)
func FirestoreInitClient(projectID string, credPath ...string) error {
	opts := []option.ClientOption{}
	if len(credPath) > 0 && credPath[0] != "" {
		opts = append(opts, option.WithCredentialsFile(credPath[0]))
	}

	client, err := firestore.NewClient(context.Background(), projectID, opts...)
	if err != nil {
		return err
	}
	firestoreClient = client
	return nil
}

// firestoreReady checks if Firestore is initialized and writes an error if not.
func firestoreReady(w http.ResponseWriter) bool {
	if firestoreClient == nil {
		respond(w, 500, false, "Firestore not initialized. Call grit.FirestoreInit(projectID) first.", nil)
		return false
	}
	return true
}

// ========================
// Internal Model Helpers
// ========================

// firestoreMapToResult converts a Firestore document map to a registered
// model struct (if one is registered for 'name'), otherwise returns the
// raw map. Uses a JSON bridge: map → JSON bytes → struct pointer.
func firestoreMapToResult(data map[string]interface{}, name string) interface{} {
	model := models[name]
	if model == nil {
		return data
	}
	b, _ := json.Marshal(data)
	obj := clone(model)
	_ = json.Unmarshal(b, obj)
	return obj
}

// firestoreBodyToMap decodes the HTTP request body.
// If a model is registered for 'name', the body is decoded into that
// model struct first (for validation / field filtering), then bridged
// back to map[string]interface{} for Firestore storage.
// If no model is registered, the body is decoded directly to a raw map.
func firestoreBodyToMap(r *http.Request, name string) (map[string]interface{}, error) {
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

// ========================
// CREATE — POST → collection.Add()
// ========================

// FirestoreC creates a new document in a Firestore collection.
// Firestore auto-generates the document ID.
//
// If a model is registered for 'name' via grit.RegisterModel(), the
// request body is decoded into that model for validation before storing.
//
// Usage (with model):
//
//	grit.RegisterModel("posts", &Post{})
//	r.Post("/posts", auth(grit.FirestoreC("posts")))
//
// Usage (without model — raw map):
//
//	r.Post("/posts", auth(grit.FirestoreC("posts")))
//
// Body:   { "title": "Hello", "author": "Milan" }
func FirestoreC(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			methodNotAllowed(w, r, "POST")
			return
		}

		if !firestoreReady(w) {
			return
		}

		data, err := firestoreBodyToMap(r, name)
		if err != nil {
			respond(w, 400, false, "Invalid request body", nil)
			return
		}

		if len(data) == 0 {
			respond(w, 400, false, "Request body cannot be empty", nil)
			return
		}

		ref, _, err := firestoreClient.Collection(name).Add(context.Background(), data)
		if err != nil {
			respond(w, 500, false, "Firestore error: "+err.Error(), nil)
			return
		}

		respond(w, 201, true, "Created successfully", map[string]interface{}{
			"id": ref.ID,
		})
	}
}

// ========================
// READ ALL — GET → collection.Documents()
// ========================

// FirestoreR fetches all documents from a Firestore collection.
// Each document includes its Firestore document ID under the "id" field.
//
// If a model is registered for 'name', each document is decoded into
// that model struct and returned as a typed array.
//
// Usage:
//
//	grit.RegisterModel("posts", &Post{})
//	r.Get("/posts", auth(grit.FirestoreR("posts")))
func FirestoreR(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodGet {
			methodNotAllowed(w, r, "GET")
			return
		}

		if !firestoreReady(w) {
			return
		}

		iter := firestoreClient.Collection(name).Documents(context.Background())
		defer iter.Stop()

		var results []interface{}

		for {
			doc, err := iter.Next()
			if err != nil {
				break // iterator.Done or real error — both exit cleanly
			}
			data := doc.Data()
			data["id"] = doc.Ref.ID // inject document ID
			results = append(results, firestoreMapToResult(data, name))
		}

		if results == nil {
			results = []interface{}{} // return empty array, not null
		}

		respond(w, 200, true, "Fetched successfully", results)
	}
}

// ========================
// READ BY ID — GET → collection.Doc(id).Get()
// ========================

// FirestoreGetByID fetches a single document from a Firestore collection by its ID.
// If a model is registered for 'name', the document is decoded into that model struct.
//
// Usage:
//
//	grit.RegisterModel("posts", &Post{})
//	r.Get("/post", auth(grit.FirestoreGetByID("posts")))
//
// Query:  GET /post?id=<documentID>
func FirestoreGetByID(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodGet {
			methodNotAllowed(w, r, "GET")
			return
		}

		if !firestoreReady(w) {
			return
		}

		id := r.URL.Query().Get("id")
		if id == "" {
			respond(w, 400, false, "id query parameter is required", nil)
			return
		}

		doc, err := firestoreClient.Collection(name).Doc(id).Get(context.Background())
		if err != nil {
			respond(w, 404, false, "Document not found", nil)
			return
		}

		data := doc.Data()
		data["id"] = doc.Ref.ID

		respond(w, 200, true, "Fetched successfully", firestoreMapToResult(data, name))
	}
}

// ========================
// UPDATE (MERGE) — PUT/PATCH → collection.Doc(id).Set(data, MergeAll)
// ========================

// FirestoreU updates (merges) fields on an existing Firestore document.
// Requires an "id" field in the JSON body (the Firestore document ID).
// Only the provided fields are updated — other fields are left untouched.
//
// If a model is registered for 'name', the body is decoded into the model
// struct first, filtering out any unknown fields before updating.
//
// Usage:
//
//	grit.RegisterModel("posts", &Post{})
//	r.Put("/post",   auth(grit.FirestoreU("posts")))
//	r.Patch("/post", auth(grit.FirestoreU("posts")))
//
// Body:   { "id": "<docID>", "title": "updated title" }
func FirestoreU(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPut && r.Method != http.MethodPatch {
			methodNotAllowed(w, r, "PUT or PATCH")
			return
		}

		if !firestoreReady(w) {
			return
		}

		// Always decode as raw map first so we can extract "id"
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			respond(w, 400, false, "Invalid request body", nil)
			return
		}

		idRaw, ok := payload["id"]
		if !ok {
			respond(w, 400, false, "id field is required in body", nil)
			return
		}

		id, ok := idRaw.(string)
		if !ok || id == "" {
			respond(w, 400, false, "id must be a non-empty string", nil)
			return
		}

		delete(payload, "id") // never store "id" as a Firestore field

		if len(payload) == 0 {
			respond(w, 400, false, "No fields to update", nil)
			return
		}

		// If a model is registered, filter payload through the model struct
		// so only known fields are updated (prevents unknown field injection).
		model := models[name]
		if model != nil {
			b, _ := json.Marshal(payload)
			obj := clone(model)
			_ = json.Unmarshal(b, obj)
			// Bridge back to map
			b2, _ := json.Marshal(obj)
			var filtered map[string]interface{}
			_ = json.Unmarshal(b2, &filtered)
			payload = filtered
		}

		// MergeAll — only provided fields are updated
		_, err := firestoreClient.Collection(name).Doc(id).Set(
			context.Background(),
			payload,
			firestore.MergeAll,
		)
		if err != nil {
			respond(w, 500, false, "Firestore error: "+err.Error(), nil)
			return
		}

		respond(w, 200, true, "Updated successfully", nil)
	}
}

// ========================
// DELETE — DELETE → collection.Doc(id).Delete()
// ========================

// FirestoreD deletes a document from a Firestore collection by its ID.
// Requires an "id" field in the JSON body.
//
// Usage:
//
//	r.Delete("/post", auth(grit.FirestoreD("posts")))
//
// Body:   { "id": "<docID>" }
func FirestoreD(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodDelete {
			methodNotAllowed(w, r, "DELETE")
			return
		}

		if !firestoreReady(w) {
			return
		}

		var body struct {
			ID string `json:"id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
			respond(w, 400, false, "id is required in body", nil)
			return
		}

		_, err := firestoreClient.Collection(name).Doc(body.ID).Delete(context.Background())
		if err != nil {
			respond(w, 500, false, "Firestore error: "+err.Error(), nil)
			return
		}

		respond(w, 200, true, "Deleted successfully", nil)
	}
}

// ========================
// QUERY BY FIELD — GET → collection.Where(field, op, value)
// ========================

// FirestoreWhere returns all documents where a given field matches a value.
// If a model is registered for 'name', each result is decoded into that struct.
//
// Usage:
//
//	grit.RegisterModel("posts", &Post{})
//	r.Get("/posts/by-author", auth(grit.FirestoreWhere("posts", "author", "==")))
//
// Query:  GET /posts/by-author?value=Milan
//
// Supported operators: "==", "!=", "<", "<=", ">", ">="
func FirestoreWhere(name, field, operator string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodGet {
			methodNotAllowed(w, r, "GET")
			return
		}

		if !firestoreReady(w) {
			return
		}

		value := r.URL.Query().Get("value")
		if value == "" {
			respond(w, 400, false, "value query parameter is required", nil)
			return
		}

		iter := firestoreClient.Collection(name).
			Where(field, operator, value).
			Documents(context.Background())
		defer iter.Stop()

		var results []interface{}

		for {
			doc, err := iter.Next()
			if err != nil {
				break
			}
			data := doc.Data()
			data["id"] = doc.Ref.ID
			results = append(results, firestoreMapToResult(data, name))
		}

		if results == nil {
			results = []interface{}{}
		}

		respond(w, 200, true, "Fetched successfully", results)
	}
}
