package grit

import (
	"context"
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

/*
================================================
 Context Keys (SAFE, COLLISION-FREE)
================================================
*/

type MongoContextKey string

const (
	MongoUserIDKey      MongoContextKey = "mongo_user_id"
	MongoPermissionsKey MongoContextKey = "mongo_permissions"
	MongoEmailKey       MongoContextKey = "mongo_email"
)

/*
================================================
 MongoProtected Middleware
================================================

✔ Validates Authorization header
✔ Validates Bearer token format
✔ Validates JWT signature + expiry
✔ Extracts user ID, email, permissions
✔ Injects values into request context
✔ DOES NOT modify existing auth functions
*/

func MongoProtected(jwtSecret string) func(http.HandlerFunc) http.HandlerFunc {

	secret := []byte(jwtSecret)

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {

			// ---------- Authorization Header ----------
			auth := r.Header.Get("Authorization")
			if auth == "" {
				http.Error(w, "Authorization required", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(auth, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid token format", http.StatusUnauthorized)
				return
			}

			// ---------- Parse JWT ----------
			token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {

				// Enforce HMAC
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}

				return secret, nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			// ---------- Extract Claims ----------
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "Invalid token claims", http.StatusUnauthorized)
				return
			}

			// ---------- User ID ----------
			sub, ok := claims["sub"].(string)
			if !ok {
				http.Error(w, "Invalid user id", http.StatusUnauthorized)
				return
			}

			userID, err := primitive.ObjectIDFromHex(sub)
			if err != nil {
				http.Error(w, "Invalid user id format", http.StatusUnauthorized)
				return
			}

			// ---------- Permissions ----------
			rawPerms, ok := claims["permissions"].([]interface{})
			if !ok {
				http.Error(w, "Permissions missing", http.StatusForbidden)
				return
			}

			perms := make([]string, 0)
			for _, p := range rawPerms {
				if s, ok := p.(string); ok {
					perms = append(perms, s)
				}
			}

			// ---------- Email (optional) ----------
			email, _ := claims["email"].(string)

			// ---------- Inject into Context ----------
			ctx := context.WithValue(r.Context(), MongoUserIDKey, userID)
			ctx = context.WithValue(ctx, MongoPermissionsKey, perms)
			ctx = context.WithValue(ctx, MongoEmailKey, email)

			// ---------- Continue ----------
			next(w, r.WithContext(ctx))
		}
	}
}
