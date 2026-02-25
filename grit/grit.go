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

// ========================
// Firebase Auth
// ========================

// FirebaseInit initializes the Firebase Admin SDK using a service account JSON file.
// For a combined Firebase + Firestore init, use grit.InitFirebase() instead.
var FirebaseInit = FirebaseInitAdmin

// InitFirebase initializes BOTH Firebase Admin SDK AND Firestore in one call.
// It handles all error checking, logging, and calls log.Fatal on failure.
// This is the recommended way to boot Firebase in your main():
//
//	grit.InitFirebase("serviceAccountKey.json", "your-project-id")
//
// InitFirebase is defined in firebase_auth.go.

// FirebaseSignup creates a user in Firebase and issues an app JWT.
var FirebaseSignup = FirebaseSignupHandler

// FirebaseSignin verifies a Firebase ID Token (from client SDK) and issues an app JWT.
var FirebaseSignin = FirebaseSigninHandler

// FirebaseMe returns the authenticated user's profile from the JWT context.
// Route must be wrapped with FirebaseProtected middleware:
//
//	r.Get("/auth/me", grit.FirebaseProtected(secret)(grit.FirebaseMe))
//
// FirebaseMeHandler is defined in firebase_auth.go.

// ========================
// Supabase
// ========================

// SupabaseInit stores the Supabase project URL and API key.
var SupabaseInit = SupabaseInitClient

// SupabaseSignup signs up via Supabase Auth REST API and issues an app JWT.
var SupabaseSignup = SupabaseSignupHandler

// SupabaseSignin signs in via Supabase Auth REST API and issues an app JWT.
var SupabaseSignin = SupabaseSigninHandler

// ========================
// Firestore CRUD
// ========================

// FirestoreInit initializes the Firestore client using your Firebase project ID.
// Must be called after FirebaseInit(). For combined init, use grit.InitFirebase().
var FirestoreInit = FirestoreInitClient

// FirestoreC, FirestoreR, FirestoreGetByID, FirestoreU, FirestoreD, FirestoreWhere
// are all defined in grit/firestore.go and are ready to use directly.

// ========================
// Utility Handlers
// ========================

// Health is a ready-to-use health check handler.
//
//	r.Get("/health", grit.Health)
//
// HealthHandler is defined in helpers.go.
