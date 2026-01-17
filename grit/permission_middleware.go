package grit

import (
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
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
				return secret, nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			claims := token.Claims.(jwt.MapClaims)
			rawPerms := claims["permissions"].([]interface{})

			for _, p := range rawPerms {
				perm := p.(string)
				if perm == required || perm == "admin:all" {
					next(w, r)
					return
				}
			}

			http.Error(w, "Forbidden", http.StatusForbidden)
		}
	}
}
