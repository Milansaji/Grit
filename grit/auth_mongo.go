package grit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/mail"
	"time"

	"github.com/dgrijalva/jwt-go"
	"golang.org/x/crypto/bcrypt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

/* =========================
   MODEL
========================= */

type AuthUser struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email        string             `bson:"email" json:"email"`
	PasswordHash string             `bson:"password_hash" json:"-"`
	Permissions  []string           `bson:"permissions,omitempty" json:"permissions"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
}

/* =========================
   RESPONSE
========================= */

func authRespond(w http.ResponseWriter, code int, success bool, msg string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": success,
		"message": msg,
		"data":    data,
	})
}

/* =========================
   SIGNUP
========================= */

func CreateUserWithEmailAndPasswordMongo(jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			methodNotAllowed(w, r, "POST")
			return
		}

		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			authRespond(w, 400, false, "Invalid request body", nil)
			return
		}

		if body.Email == "" || body.Password == "" {
			authRespond(w, 400, false, "Email and password required", nil)
			return
		}

		if _, err := mail.ParseAddress(body.Email); err != nil {
			authRespond(w, 400, false, "Invalid email format", nil)
			return
		}

		if len(body.Password) < 6 {
			authRespond(w, 400, false, "Password must be >= 6 characters", nil)
			return
		}

		col := mongoDB.Collection("users")

		// duplicate check
		if count, _ := col.CountDocuments(context.Background(), bson.M{"email": body.Email}); count > 0 {
			authRespond(w, 409, false, "Email already exists", nil)
			return
		}

		// first user = admin
		totalUsers, _ := col.CountDocuments(context.Background(), bson.M{})
		permissions := []string{"user:read"}

		if totalUsers == 0 {
			permissions = []string{"admin:all"}
		}

		hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)

		user := AuthUser{
			ID:           primitive.NewObjectID(),
			Email:        body.Email,
			PasswordHash: string(hash),
			Permissions:  permissions,
			CreatedAt:    time.Now(),
		}

		col.InsertOne(context.Background(), user)

		token := buildJWT(user, jwtSecret)

		user.PasswordHash = ""

		authRespond(w, 201, true, "Signup successful", map[string]interface{}{
			"token": token,
			"user":  user,
		})
	}
}

/* =========================
   SIGNIN (FIXED)
========================= */

func SigninUserWithEmailAndPassMongo(jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			authRespond(w, 400, false, "Invalid request body", nil)
			return
		}

		col := mongoDB.Collection("users")

		var user AuthUser
		if err := col.FindOne(context.Background(), bson.M{"email": body.Email}).Decode(&user); err != nil {
			authRespond(w, 401, false, "Invalid email or password", nil)
			return
		}

		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)) != nil {
			authRespond(w, 401, false, "Invalid email or password", nil)
			return
		}

		// 🔥 FIX: fallback permissions for OLD USERS
		if len(user.Permissions) == 0 {
			user.Permissions = []string{"user:read"}
			col.UpdateOne(
				context.Background(),
				bson.M{"_id": user.ID},
				bson.M{"$set": bson.M{"permissions": user.Permissions}},
			)
		}

		token := buildJWT(user, jwtSecret)

		user.PasswordHash = ""

		authRespond(w, 200, true, "Signin successful", map[string]interface{}{
			"token": token,
			"user":  user,
		})
	}
}

/* =========================
   JWT BUILDER (SINGLE SOURCE)
========================= */

func buildJWT(user AuthUser, secret string) string {
	claims := jwt.MapClaims{
		"sub":         user.ID.Hex(),
		"email":       user.Email,
		"permissions": user.Permissions,
		"exp":         time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(secret))
	return signed
}

/* =========================
   GET ALL USERS
========================= */

func GetAllUsersMongo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		col := mongoDB.Collection("users")

		var users []AuthUser
		cursor, _ := col.Find(context.Background(), bson.M{})
		cursor.All(context.Background(), &users)

		for i := range users {
			users[i].PasswordHash = ""
			if len(users[i].Permissions) == 0 {
				users[i].Permissions = []string{"user:read"}
			}
		}

		authRespond(w, 200, true, "Users fetched successfully", users)
	}
}

func DeleteUserMongo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodDelete {
			methodNotAllowed(w, r, "DELETE")
			return
		}

		var body struct {
			ID string `json:"id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
			authRespond(w, 400, false, "User ID required", nil)
			return
		}

		objID, err := primitive.ObjectIDFromHex(body.ID)
		if err != nil {
			authRespond(w, 400, false, "Invalid ID format", nil)
			return
		}

		col := mongoDB.Collection("users")

		res, err := col.DeleteOne(
			context.Background(),
			bson.M{"_id": objID},
		)

		if err != nil {
			authRespond(w, 500, false, err.Error(), nil)
			return
		}

		if res.DeletedCount == 0 {
			authRespond(w, 404, false, "User not found", nil)
			return
		}

		authRespond(w, 200, true, "User deleted successfully", nil)
	}
}
