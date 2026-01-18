package grit

import "net/http"

// Router
var NewRouter = New

// Middleware
var Protect = ProtectSQLite

// SQLite Auth
var SignupSQLite = SignupSQLiteHandler
var SigninSQLite = SigninSQLiteHandler

// Mongo Auth
var SignupMongo = CreateUserWithEmailAndPasswordMongo
var SigninMongo = SigninUserWithEmailAndPassMongo

// Mongo Init
var MongoConnect = MongoInit

// Role helpers
var RequireAdmin = func(secret string) func(http.HandlerFunc) http.HandlerFunc {
	return RequireRole(secret, "admin")
}
