package grit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/mail"
	"strings"
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
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
}

/* =========================
   INTERNAL RESPONSE
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
   SIGNUP + JWT
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

		if mongoDB == nil {
			authRespond(w, 500, false, "MongoDB not initialized", nil)
			return
		}

		col := mongoDB.Collection("users")

		// check duplicate
		count, err := col.CountDocuments(
			context.Background(),
			bson.M{"email": body.Email},
		)
		if err != nil {
			authRespond(w, 500, false, err.Error(), nil)
			return
		}

		if count > 0 {
			authRespond(w, 409, false, "Email already exists", nil)
			return
		}

		hash, err := bcrypt.GenerateFromPassword(
			[]byte(body.Password),
			bcrypt.DefaultCost,
		)
		if err != nil {
			authRespond(w, 500, false, "Password hashing failed", nil)
			return
		}

		user := AuthUser{
			ID:           primitive.NewObjectID(),
			Email:        body.Email,
			PasswordHash: string(hash),
			CreatedAt:    time.Now(),
		}

		if _, err := col.InsertOne(context.Background(), user); err != nil {
			authRespond(w, 500, false, "Failed to create user", nil)
			return
		}

		claims := jwt.MapClaims{
			"sub":   user.ID.Hex(),
			"email": user.Email,
			"iat":   time.Now().Unix(),
			"exp":   time.Now().Add(24 * time.Hour).Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := token.SignedString([]byte(jwtSecret))
		if err != nil {
			authRespond(w, 500, false, "Token generation failed", nil)
			return
		}

		user.PasswordHash = ""

		authRespond(w, 201, true, "Signup successful", map[string]interface{}{
			"token": signed,
			"user":  user,
		})
	}
}

/* =========================
   SIGNIN + JWT
========================= */

func SigninUserWithEmailAndPassMongo(jwtSecret string) http.HandlerFunc {
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

		col := mongoDB.Collection("users")

		var user AuthUser
		err := col.FindOne(
			context.Background(),
			bson.M{"email": body.Email},
		).Decode(&user)

		if err != nil {
			authRespond(w, 401, false, "Invalid email or password", nil)
			return
		}

		if err := bcrypt.CompareHashAndPassword(
			[]byte(user.PasswordHash),
			[]byte(body.Password),
		); err != nil {
			authRespond(w, 401, false, "Invalid email or password", nil)
			return
		}

		claims := jwt.MapClaims{
			"sub":   user.ID.Hex(),
			"email": user.Email,
			"iat":   time.Now().Unix(),
			"exp":   time.Now().Add(24 * time.Hour).Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, _ := token.SignedString([]byte(jwtSecret))

		user.PasswordHash = ""

		authRespond(w, 200, true, "Signin successful", map[string]interface{}{
			"token": signed,
			"user":  user,
		})
	}
}

/* =========================
   GET ALL USERS
========================= */

func GetAllUsersMongo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodGet {
			methodNotAllowed(w, r, "GET")
			return
		}

		col := mongoDB.Collection("users")

		var users []AuthUser

		cursor, err := col.Find(context.Background(), bson.M{})
		if err != nil {
			authRespond(w, 500, false, err.Error(), nil)
			return
		}

		if err := cursor.All(context.Background(), &users); err != nil {
			authRespond(w, 500, false, err.Error(), nil)
			return
		}

		for i := range users {
			users[i].PasswordHash = ""
		}

		authRespond(w, 200, true, "Users fetched successfully", users)
	}
}

/* =========================
   GET USER BY ID
========================= */

func GetUserByIDMongo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			methodNotAllowed(w, r, "POST")
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

		var user AuthUser
		err = col.FindOne(
			context.Background(),
			bson.M{"_id": objID},
		).Decode(&user)

		if err != nil {
			authRespond(w, 404, false, "User not found", nil)
			return
		}

		user.PasswordHash = ""

		authRespond(w, 200, true, "User fetched successfully", user)
	}
}

/* =========================
   DELETE USER
========================= */

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

/* =========================
   AUTH PROTECTED (JWT)
========================= */

func AuthProtectedMongo(jwtSecret string) func(http.HandlerFunc) http.HandlerFunc {

	secret := []byte(jwtSecret)

	return func(next http.HandlerFunc) http.HandlerFunc {

		return func(w http.ResponseWriter, r *http.Request) {

			auth := r.Header.Get("Authorization")
			if auth == "" {
				http.Error(w, "Authorization token required", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(auth, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid token format", http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {

				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}

				return secret, nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			next(w, r)
		}
	}
}
