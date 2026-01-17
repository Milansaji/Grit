package grit

import (
	"encoding/json"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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
		return err
	}

	if err := db.AutoMigrate(&User{}); err != nil {
		return err
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
	json.NewEncoder(w).Encode(user)
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

		json.NewDecoder(r.Body).Decode(&body)

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

		json.NewEncoder(w).Encode(map[string]interface{}{
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

			token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {
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
