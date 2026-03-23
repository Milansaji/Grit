package grit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

/* =========================
   TOKEN REVOCATION (SQLite)
========================= */

type Revocation struct {
	Token     string `gorm:"primaryKey"`
	CreatedAt time.Time
}

func sqliteBlacklistAdd(token string) {
	if sqliteDB == nil {
		InitSQLite()
	}
	sqliteDB.Create(&Revocation{
		Token:     token,
		CreatedAt: time.Now(),
	})
}

func sqliteBlacklistContains(token string) bool {
	if sqliteDB == nil {
		InitSQLite()
	}
	var count int64
	sqliteDB.Model(&Revocation{}).Where("token = ?", token).Count(&count)
	return count > 0
}

/* =========================
   MODEL
========================= */

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Email        string    `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string    `json:"-"`
	Permissions  string    `gorm:"not null" json:"permissions"` // CSV
	CreatedAt    time.Time `json:"created_at"`
}

/* =========================
   DB
========================= */

var sqliteDB *gorm.DB

func InitSQLite() error {
	if sqliteDB != nil {
		return nil
	}

	db, err := gorm.Open(sqlite.Open("auth.db"), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to open auth.db: %w — ensure the process has write permission to the working directory", err)
	}

	// WAL mode — better concurrency, prevents file-locking on multi-goroutine access
	db.Exec("PRAGMA journal_mode=WAL;")

	if err := db.AutoMigrate(&User{}, &Revocation{}); err != nil {
		return fmt.Errorf("AutoMigrate failed: %w", err)
	}

	sqliteDB = db
	return nil
}

/* =========================
   HELPERS
========================= */

func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error": msg,
	})
}

func splitPermissions(csv string) []string {
	if csv == "" {
		return []string{}
	}
	return strings.Split(csv, ",")
}

/* =========================
   SIGNUP
========================= */

func SignupSQLiteHandler(w http.ResponseWriter, r *http.Request) {

	if err := InitSQLite(); err != nil {
		writeJSONError(w, 500, "database error")
		return
	}

	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, 400, "invalid body")
		return
	}

	if _, err := mail.ParseAddress(body.Email); err != nil {
		writeJSONError(w, 400, "invalid email")
		return
	}

	if len(body.Password) < 6 {
		writeJSONError(w, 400, "password too short")
		return
	}

	var count int64
	sqliteDB.Model(&User{}).Count(&count)

	permissions := "user:read"
	if count == 0 {
		permissions = "admin:all"
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)

	user := User{
		Email:        body.Email,
		PasswordHash: string(hash),
		Permissions:  permissions,
		CreatedAt:    time.Now(),
	}

	if err := sqliteDB.Create(&user).Error; err != nil {
		writeJSONError(w, 409, "email exists")
		return
	}

	user.PasswordHash = ""

	respond(w, 201, true, "Signup successful", user)
}

/* =========================
   SIGNIN + JWT
========================= */

func SigninSQLiteHandler(jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if err := InitSQLite(); err != nil {
			writeJSONError(w, 500, "database error")
			return
		}

		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, 400, "invalid body")
			return
		}

		var user User
		if err := sqliteDB.Where("email = ?", body.Email).First(&user).Error; err != nil {
			writeJSONError(w, 401, "invalid credentials")
			return
		}

		if bcrypt.CompareHashAndPassword(
			[]byte(user.PasswordHash),
			[]byte(body.Password),
		) != nil {
			writeJSONError(w, 401, "invalid credentials")
			return
		}

		perms := splitPermissions(user.Permissions)

		claims := jwt.MapClaims{
			"sub":         user.ID,
			"email":       user.Email,
			"permissions": perms,
			"exp":         time.Now().Add(24 * time.Hour).Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, _ := token.SignedString([]byte(jwtSecret))

		user.PasswordHash = ""

		respond(w, 200, true, "Signin successful", map[string]interface{}{
			"token": signed,
			"user":  user,
		})
	}
}

/* =========================
   JWT PROTECT
========================= */

func ProtectSQLite(jwtSecret string) func(http.HandlerFunc) http.HandlerFunc {

	secret := []byte(jwtSecret)

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {

			auth := r.Header.Get("Authorization")
			if auth == "" {
				http.Error(w, "authorization required", 401)
				return
			}

			parts := strings.Split(auth, " ")
			if len(parts) != 2 {
				http.Error(w, "invalid token", 401)
				return
			}

			rawToken := parts[1]

			// Reject blacklisted (signed-out) tokens
			if sqliteBlacklistContains(rawToken) {
				http.Error(w, "token revoked", 401)
				return
			}

			token, err := jwt.Parse(rawToken, func(t *jwt.Token) (interface{}, error) {
				return secret, nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "invalid token", 401)
				return
			}

			next(w, r)
		}
	}
}

/* =========================
   SIGNOUT
========================= */

// SignoutSQLiteHandler invalidates the current JWT by adding it to the
// in-memory blacklist. The client should discard the token after this call.
//
// Usage:
//
//	r.Post("/auth/signout", grit.ProtectSQLite(secret)(grit.SignoutSQLiteHandler))
func SignoutSQLiteHandler(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	parts := strings.Split(auth, " ")
	if len(parts) == 2 {
		sqliteBlacklistAdd(parts[1])
	}
	respond(w, 200, true, "Signed out successfully", nil)
}

/* =========================
   GET ALL USERS (SQLITE)
========================= */

func GetAllUsersSQLiteHandler(w http.ResponseWriter, r *http.Request) {

	if err := InitSQLite(); err != nil {
		writeJSONError(w, 500, "database error")
		return
	}

	var users []User

	if err := sqliteDB.Find(&users).Error; err != nil {
		writeJSONError(w, 500, "failed to fetch users")
		return
	}

	for i := range users {
		users[i].PasswordHash = ""
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Users fetched successfully",
		"data":    users,
	})
}

/* =========================
   GET USER BY ID (SQLITE)
========================= */

func GetUserByIDSQLiteHandler(w http.ResponseWriter, r *http.Request) {

	if err := InitSQLite(); err != nil {
		writeJSONError(w, 500, "database error")
		return
	}

	// Read id from query param
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSONError(w, 400, "user id is required")
		return
	}

	var user User

	if err := sqliteDB.First(&user, id).Error; err != nil {
		writeJSONError(w, 404, "user not found")
		return
	}

	user.PasswordHash = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "User fetched successfully",
		"data":    user,
	})
}
