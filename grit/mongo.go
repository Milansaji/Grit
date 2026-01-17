package grit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	mongoClient *mongo.Client
	mongoDB     *mongo.Database
)

// MongoInit initializes MongoDB connection
func MongoInit(uri string, dbName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}

	if err := client.Ping(ctx, nil); err != nil {
		return err
	}

	mongoClient = client
	mongoDB = client.Database(dbName)

	fmt.Println("✅ MongoDB connected:", dbName)
	return nil
}

var mongoCollections = map[string]*mongo.Collection{}

// MongoCollection binds a registered model to MongoDB collection
func MongoCollection(name string) (*mongo.Collection, error) {
	if mongoDB == nil {
		return nil, fmt.Errorf("MongoDB not initialized")
	}

	model := models[name]
	if model == nil {
		return nil, fmt.Errorf("model not registered: %s", name)
	}

	col := mongoDB.Collection(name)
	mongoCollections[name] = col

	return col, nil
}

// ---------- CREATE ----------
func MongoC(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			methodNotAllowed(w, r, "POST")
			return
		}

		col, err := MongoCollection(name)
		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}

		model := models[name]
		obj := clone(model)

		if err := json.NewDecoder(r.Body).Decode(obj); err != nil {
			respond(w, 400, false, "Invalid body", nil)
			return
		}

		res, err := col.InsertOne(context.Background(), obj)
		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}

		respond(w, 201, true, "Created", res.InsertedID)
	}
}

// ---------- READ ALL ----------
func MongoR(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodGet {
			methodNotAllowed(w, r, "GET")
			return
		}

		col, err := MongoCollection(name)
		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}

		model := models[name]
		slice := makeSlice(model)

		cursor, err := col.Find(context.Background(), map[string]interface{}{})
		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}

		if err := cursor.All(context.Background(), slice); err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}

		respond(w, 200, true, "Fetched", slice)
	}
}

// ---------- READ BY ID ----------
func MongoGetByID(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			methodNotAllowed(w, r, "POST")
			return
		}

		var body struct {
			ID string `json:"id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
			respond(w, 400, false, "ID is required", nil)
			return
		}

		col, err := MongoCollection(name)
		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}

		objID, err := primitive.ObjectIDFromHex(body.ID)
		if err != nil {
			respond(w, 400, false, "Invalid ID format", nil)
			return
		}

		model := models[name]
		obj := clone(model)

		err = col.FindOne(
			context.Background(),
			bson.M{"_id": objID},
		).Decode(obj)

		if err != nil {
			respond(w, 404, false, "Record not found", nil)
			return
		}

		respond(w, 200, true, "Fetched successfully", obj)
	}
}

// ---------- UPDATE ----------
func MongoU(name string) http.HandlerFunc {
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

		idStr, ok := idRaw.(string)
		if !ok {
			respond(w, 400, false, "Invalid ID type", nil)
			return
		}

		objID, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			respond(w, 400, false, "Invalid ID format", nil)
			return
		}

		delete(payload, "id") // never update _id

		col, err := MongoCollection(name)
		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}

		result, err := col.UpdateOne(
			context.Background(),
			bson.M{"_id": objID},
			bson.M{"$set": payload},
		)

		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}

		if result.MatchedCount == 0 {
			respond(w, 404, false, "Record not found", nil)
			return
		}

		respond(w, 200, true, "Updated successfully", nil)
	}
}

// ---------- DELETE ----------
func MongoD(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodDelete {
			methodNotAllowed(w, r, "DELETE")
			return
		}

		var body struct {
			ID string `json:"id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respond(w, 400, false, "Invalid ID", nil)
			return
		}

		col, err := MongoCollection(name)
		if err != nil {
			respond(w, 500, false, err.Error(), nil)
			return
		}

		objID, _ := primitive.ObjectIDFromHex(body.ID)
		res, _ := col.DeleteOne(
			context.Background(),
			bson.M{"_id": objID},
		)

		if res.DeletedCount == 0 {
			respond(w, 404, false, "Not found", nil)
			return
		}

		respond(w, 200, true, "Deleted", nil)
	}
}
