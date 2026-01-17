package grit

import "net/http"

// Router
var NewRouter = New

// Middleware

// TODO: Implement AuthProtected or import it from the correct package
// Example placeholder implementation:
var AuthProtected = func(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Authentication logic goes here
		next(w, r)
	}
}

var Protect = AuthProtected
var ProtectMongo = AuthProtectedMongo

// SQLite Auth
var SignupSQLite = CreateUserWithEmailAndPassword
var SigninSQLite = SigninUserWithEmailAndPass

// Mongo Auth
var SignupMongo = CreateUserWithEmailAndPasswordMongo
var SigninMongo = SigninUserWithEmailAndPassMongo

// Mongo Init
var MongoConnect = MongoInit

// Role helpers
var RequireAdmin = func(secret string) func(http.HandlerFunc) http.HandlerFunc {
	return RequireRole(secret, "admin")
}
