package grit

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v4"
)

// RequireRole enforces role-based access
func RequireRole(jwtSecret string, requiredRole string) func(http.HandlerFunc) http.HandlerFunc {

	secret := []byte(jwtSecret)

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {

			auth := r.Header.Get("Authorization")
			if auth == "" {
				http.Error(w, "authorization required", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(auth, " ")
			if len(parts) != 2 {
				http.Error(w, "invalid token format", http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {
				return secret, nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			claims := token.Claims.(jwt.MapClaims)
			role, ok := claims["role"].(string)
			if !ok || role != requiredRole {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			next(w, r)
		}
	}
}
