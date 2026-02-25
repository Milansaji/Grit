package grit

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v4"
)

func RequirePermission(jwtSecret string, required string) func(http.HandlerFunc) http.HandlerFunc {

	secret := []byte(jwtSecret)

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {

			auth := r.Header.Get("Authorization")
			if auth == "" {
				http.Error(w, "Authorization required", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(auth, " ")
			if len(parts) != 2 {
				http.Error(w, "Invalid token format", http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {
				// Enforce HMAC to prevent algorithm confusion attacks
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return secret, nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "Invalid token claims", http.StatusUnauthorized)
				return
			}

			rawPerms, ok := claims["permissions"].([]interface{})
			if !ok {
				http.Error(w, "Permissions missing", http.StatusForbidden)
				return
			}

			for _, p := range rawPerms {
				if perm, ok := p.(string); ok {
					if perm == required || perm == "admin:all" {
						next(w, r)
						return
					}
				}
			}

			http.Error(w, "Forbidden", http.StatusForbidden)
		}
	}
}
