package grit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"cloud.google.com/go/firestore"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Store defines the unified interface for database operations.
type Store interface {
	Create(obj interface{}) error
	BulkCreate(objs interface{}) error
	ReadAll(slice interface{}) error
	GetByID(id interface{}, obj interface{}) error
	Update(id interface{}, payload map[string]interface{}) error
	Delete(id interface{}) error
	GetModel() interface{}
}

// Global configuration for the default store type
var defaultStoreType = "sqlite"

// SetDefaultStore changes the database backend used by the unified CRUD handlers.
// Options: "sqlite", "mongo", "firestore", "supabase"
func SetDefaultStore(t string) {
	defaultStoreType = t
}

// ---------------------------------------------------------
// SQLiteStore Implementation (GORM)
// ---------------------------------------------------------
type SQLiteStore struct {
	name  string
	model interface{}
}

func (s *SQLiteStore) Create(obj interface{}) error {
	db, _, err := OpenCollection(s.name)
	if err != nil {
		return err
	}
	return db.Create(obj).Error
}

func (s *SQLiteStore) BulkCreate(objs interface{}) error {
	return s.Create(objs) // GORM Create handles slices for batch insert
}

func (s *SQLiteStore) ReadAll(slice interface{}) error {
	db, _, err := OpenCollection(s.name)
	if err != nil {
		return err
	}
	return db.Find(slice).Error
}

func (s *SQLiteStore) GetByID(id interface{}, obj interface{}) error {
	db, _, err := OpenCollection(s.name)
	if err != nil {
		return err
	}
	result := db.First(obj, "id = ?", id)
	if result.RowsAffected == 0 {
		return fmt.Errorf("record not found")
	}
	return result.Error
}

func (s *SQLiteStore) Update(id interface{}, payload map[string]interface{}) error {
	db, _, err := OpenCollection(s.name)
	if err != nil {
		return err
	}
	return db.Model(clone(s.model)).Where("id = ?", id).Updates(payload).Error
}

func (s *SQLiteStore) Delete(id interface{}) error {
	db, _, err := OpenCollection(s.name)
	if err != nil {
		return err
	}
	return db.Delete(clone(s.model), "id = ?", id).Error
}

func (s *SQLiteStore) GetModel() interface{} {
	return s.model
}

// ---------------------------------------------------------
// MongoStore Implementation
// ---------------------------------------------------------
type MongoStore struct {
	name  string
	model interface{}
}

func (s *MongoStore) Create(obj interface{}) error {
	col, err := MongoCollection(s.name)
	if err != nil {
		return err
	}
	_, err = col.InsertOne(context.Background(), obj)
	return err
}

func (s *MongoStore) BulkCreate(objs interface{}) error {
	col, err := MongoCollection(s.name)
	if err != nil {
		return err
	}
	// Convert slice to []interface{}
	v := reflect.ValueOf(objs)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice {
		return fmt.Errorf("bulk create requires a slice")
	}
	var items []interface{}
	for i := 0; i < v.Len(); i++ {
		items = append(items, v.Index(i).Interface())
	}
	_, err = col.InsertMany(context.Background(), items)
	return err
}

func (s *MongoStore) ReadAll(slice interface{}) error {
	col, err := MongoCollection(s.name)
	if err != nil {
		return err
	}
	cursor, err := col.Find(context.Background(), bson.M{})
	if err != nil {
		return err
	}
	return cursor.All(context.Background(), slice)
}

func (s *MongoStore) GetByID(id interface{}, obj interface{}) error {
	col, err := MongoCollection(s.name)
	if err != nil {
		return err
	}
	idStr, ok := id.(string)
	if !ok {
		return fmt.Errorf("mongo ID must be a hex string")
	}
	objID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	return col.FindOne(context.Background(), bson.M{"_id": objID}).Decode(obj)
}

func (s *MongoStore) Update(id interface{}, payload map[string]interface{}) error {
	col, err := MongoCollection(s.name)
	if err != nil {
		return err
	}
	idStr, ok := id.(string)
	if !ok {
		return fmt.Errorf("mongo ID must be a hex string")
	}
	objID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	delete(payload, "id")
	res, err := col.UpdateOne(context.Background(), bson.M{"_id": objID}, bson.M{"$set": payload})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("record not found")
	}
	return nil
}

func (s *MongoStore) Delete(id interface{}) error {
	col, err := MongoCollection(s.name)
	if err != nil {
		return err
	}
	idStr, ok := id.(string)
	if !ok {
		return fmt.Errorf("mongo ID must be a hex string")
	}
	objID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	res, err := col.DeleteOne(context.Background(), bson.M{"_id": objID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return fmt.Errorf("record not found")
	}
	return nil
}

func (s *MongoStore) GetModel() interface{} {
	return s.model
}

// ---------------------------------------------------------
// FirestoreStore Implementation
// ---------------------------------------------------------
type FirestoreStore struct {
	name  string
	model interface{}
}

func (s *FirestoreStore) Create(obj interface{}) error {
	if firestoreClient == nil {
		return ErrFirestoreNotInitialized
	}
	// Convert obj to map for Firestore
	b, _ := json.Marshal(obj)
	var data map[string]interface{}
	json.Unmarshal(b, &data)

	_, _, err := firestoreClient.Collection(s.name).Add(context.Background(), data)
	return err
}

func (s *FirestoreStore) BulkCreate(objs interface{}) error {
	if firestoreClient == nil {
		return ErrFirestoreNotInitialized
	}
	v := reflect.ValueOf(objs)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice {
		return fmt.Errorf("bulk create requires a slice")
	}

	batch := firestoreClient.Batch()
	ctx := context.Background()
	for i := 0; i < v.Len(); i++ {
		b, _ := json.Marshal(v.Index(i).Interface())
		var data map[string]interface{}
		json.Unmarshal(b, &data)
		ref := firestoreClient.Collection(s.name).NewDoc()
		batch.Set(ref, data)
	}
	_, err := batch.Commit(ctx)
	return err
}

func (s *FirestoreStore) ReadAll(slice interface{}) error {
	if firestoreClient == nil {
		return ErrFirestoreNotInitialized
	}
	iter := firestoreClient.Collection(s.name).Documents(context.Background())
	defer iter.Stop()

	// Firestore doesn't support direct unmarshal to slice easily with doc IDs
	var items []interface{}
	for {
		doc, err := iter.Next()
		if err != nil {
			break
		}
		data := doc.Data()
		data["id"] = doc.Ref.ID
		items = append(items, firestoreMapToResult(data, s.name))
	}

	b, _ := json.Marshal(items)
	return json.Unmarshal(b, slice)
}

func (s *FirestoreStore) GetByID(id interface{}, obj interface{}) error {
	if firestoreClient == nil {
		return ErrFirestoreNotInitialized
	}
	idStr, ok := id.(string)
	if !ok {
		return fmt.Errorf("id must be a string for firestore")
	}
	doc, err := firestoreClient.Collection(s.name).Doc(idStr).Get(context.Background())
	if err != nil {
		return err
	}
	data := doc.Data()
	data["id"] = doc.Ref.ID
	b, _ := json.Marshal(data)
	return json.Unmarshal(b, obj)
}

func (s *FirestoreStore) Update(id interface{}, payload map[string]interface{}) error {
	if firestoreClient == nil {
		return ErrFirestoreNotInitialized
	}
	idStr, ok := id.(string)
	if !ok {
		return fmt.Errorf("id must be a string for firestore")
	}
	delete(payload, "id")
	_, err := firestoreClient.Collection(s.name).Doc(idStr).Set(context.Background(), payload, firestore.MergeAll)
	return err
}

func (s *FirestoreStore) Delete(id interface{}) error {
	if firestoreClient == nil {
		return ErrFirestoreNotInitialized
	}
	idStr, ok := id.(string)
	if !ok {
		return fmt.Errorf("id must be a string for firestore")
	}
	_, err := firestoreClient.Collection(s.name).Doc(idStr).Delete(context.Background())
	return err
}

func (s *FirestoreStore) GetModel() interface{} {
	return s.model
}

// ---------------------------------------------------------
// SupabaseStore Implementation
// ---------------------------------------------------------
type SupabaseStore struct {
	name  string
	model interface{}
}

func (s *SupabaseStore) Create(obj interface{}) error {
	resp, err := supabaseRequest(http.MethodPost, "/rest/v1/"+s.name, obj)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("supabase error: status %d", resp.StatusCode)
	}
	return nil
}

func (s *SupabaseStore) BulkCreate(objs interface{}) error {
	return s.Create(objs) // Supabase handles array POST
}

func (s *SupabaseStore) ReadAll(slice interface{}) error {
	resp, err := supabaseRequest(http.MethodGet, "/rest/v1/"+s.name, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("supabase error: status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(slice)
}

func (s *SupabaseStore) GetByID(id interface{}, obj interface{}) error {
	idStr := fmt.Sprintf("%v", id)
	resp, err := supabaseRequest(http.MethodGet, "/rest/v1/"+s.name+"?id=eq."+idStr+"&limit=1", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("supabase error: status %d", resp.StatusCode)
	}
	record, found := supabaseDecodeSingleResponse(resp.Body, s.name)
	if !found {
		return fmt.Errorf("record not found")
	}
	b, _ := json.Marshal(record)
	return json.Unmarshal(b, obj)
}

func (s *SupabaseStore) Update(id interface{}, payload map[string]interface{}) error {
	idStr := fmt.Sprintf("%v", id)
	delete(payload, "id")
	resp, err := supabaseRequest(http.MethodPatch, "/rest/v1/"+s.name+"?id=eq."+idStr, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("supabase error: status %d", resp.StatusCode)
	}
	return nil
}

func (s *SupabaseStore) Delete(id interface{}) error {
	idStr := fmt.Sprintf("%v", id)
	resp, err := supabaseRequest(http.MethodDelete, "/rest/v1/"+s.name+"?id=eq."+idStr, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("supabase error: status %d", resp.StatusCode)
	}
	return nil
}

func (s *SupabaseStore) GetModel() interface{} {
	return s.model
}

// ---------------------------------------------------------
// Factory & Unified Handlers
// ---------------------------------------------------------

// NewStore returns a store implementation for the given collection based on defaultStoreType.
func NewStore(name string) Store {
	key := strings.ToLower(strings.TrimSpace(name))
	model := models[key]
	if model == nil {
		return nil
	}
	switch defaultStoreType {
	case "mongo":
		return &MongoStore{name: name, model: model}
	case "firestore":
		return &FirestoreStore{name: name, model: model}
	case "supabase":
		return &SupabaseStore{name: name, model: model}
	default:
		return &SQLiteStore{name: name, model: model}
	}
}

// C (Unified Create Handler)
func C(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name = strings.ToLower(strings.TrimSpace(name))
		s := NewStore(name)
		if s == nil {
			respond(w, 500, false, "model not registered", nil)
			return
		}

		var raw json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			respond(w, 400, false, "invalid request body", nil)
			return
		}

		if len(raw) > 0 && raw[0] == '[' {
			slice := makeSlice(s.GetModel())
			if err := json.Unmarshal(raw, slice); err != nil {
				respond(w, 400, false, "invalid bulk request body", nil)
				return
			}
			if err := s.BulkCreate(slice); err != nil {
				respond(w, 500, false, err.Error(), nil)
				return
			}
			respond(w, 201, true, "bulk created successfully", slice)
		} else {
			obj := clone(s.GetModel())
			if err := json.Unmarshal(raw, obj); err != nil {
				respond(w, 400, false, "invalid request body", nil)
				return
			}
			if err := s.Create(obj); err != nil {
				respond(w, 500, false, err.Error(), nil)
				return
			}
			respond(w, 201, true, "created successfully", obj)
		}
	}
}

// R (Unified Read All Handler)
func R(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name = strings.ToLower(strings.TrimSpace(name))
		s := NewStore(name)
		if s == nil {
			respond(w, 500, false, "model not registered", nil)
			return
		}
		if r.Method != http.MethodGet {
			methodNotAllowed(w, r, "GET")
			return
		}
		slice := makeSlice(s.GetModel())
		if err := s.ReadAll(slice); err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}
		respond(w, 200, true, "fetched successfully", slice)
	}
}

// GetByID (Unified Read by ID Handler)
func GetByID(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name = strings.ToLower(strings.TrimSpace(name))
		s := NewStore(name)
		if s == nil {
			respond(w, 500, false, "model not registered", nil)
			return
		}

		var id interface{}
		if idStr := r.URL.Query().Get("id"); idStr != "" {
			id = idStr
		} else {
			var body struct {
				ID interface{} `json:"id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.ID != nil {
				id = body.ID
			}
		}

		if id == nil {
			respond(w, 400, false, "id is required", nil)
			return
		}

		obj := clone(s.GetModel())
		if err := s.GetByID(id, obj); err != nil {
			respond(w, 404, false, err.Error(), nil)
			return
		}
		respond(w, 200, true, "fetched successfully", obj)
	}
}

// U (Unified Update Handler)
func U(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name = strings.ToLower(strings.TrimSpace(name))
		s := NewStore(name)
		if s == nil {
			respond(w, 500, false, "model not registered", nil)
			return
		}
		if r.Method != http.MethodPut && r.Method != http.MethodPatch {
			methodNotAllowed(w, r, "PUT/PATCH")
			return
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			respond(w, 400, false, "invalid request body", nil)
			return
		}
		id, ok := payload["id"]
		if !ok {
			respond(w, 400, false, "id required", nil)
			return
		}
		if err := s.Update(id, payload); err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}
		respond(w, 200, true, "updated successfully", nil)
	}
}

// D (Unified Delete Handler)
func D(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name = strings.ToLower(strings.TrimSpace(name))
		s := NewStore(name)
		if s == nil {
			respond(w, 500, false, "model not registered", nil)
			return
		}
		if r.Method != http.MethodDelete {
			methodNotAllowed(w, r, "DELETE")
			return
		}

		var id interface{}
		if idStr := r.URL.Query().Get("id"); idStr != "" {
			id = idStr
		} else {
			var body struct {
				ID interface{} `json:"id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.ID != nil {
				id = body.ID
			}
		}

		if id == nil {
			respond(w, 400, false, "id required", nil)
			return
		}

		if err := s.Delete(id); err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}
		respond(w, 200, true, "deleted successfully", nil)
	}
}
