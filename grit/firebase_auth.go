package grit

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/golang-jwt/jwt/v4"
	"google.golang.org/api/option"
)

// ========================
// Firebase Context Keys
// ========================

type FirebaseContextKey string

const (
	FirebaseUIDKey         FirebaseContextKey = "firebase_uid"
	FirebaseEmailKey       FirebaseContextKey = "firebase_email"
	FirebasePermissionsKey FirebaseContextKey = "firebase_permissions"
)

// ========================
// Firebase Client (package-level)
// ========================

var firebaseAuthClient *auth.Client

// ========================
// Init
// ========================

// FirebaseInitAdmin initializes the Firebase Admin SDK using a service account JSON file.
// credPath is the path to your serviceAccountKey.json file.
func FirebaseInitAdmin(credPath string) error {
	ctx := context.Background()
	opt := option.WithCredentialsFile(credPath)

	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return err
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return err
	}

	firebaseAuthClient = client
	return nil
}

// ========================
// Combined Init (Firebase + Firestore)
// ========================

// InitFirebase initializes both the Firebase Admin SDK and Firestore in a
// single call with built-in logging and fatal error handling.
//
// This is the recommended way to initialize Firebase in your main():
//
//	grit.InitFirebase("serviceAccountKey.json", "your-project-id")
//
// credPath  — path to your serviceAccountKey.json file.
//
//	Download from: Firebase Console → Project Settings → Service Accounts.
//
// projectID — your Firebase project ID (visible in Firebase Console).
//
// Calls log.Fatal on any error so the server never starts in a broken state.
func InitFirebase(credPath, projectID string) {
	// ── Firebase Admin SDK ────────────────────────────────────
	if err := FirebaseInitAdmin(credPath); err != nil {
		if os.IsNotExist(err) {
			log.Fatalf(
				"\n❌ Firebase service account key not found at '%s'.\n"+
					"   Download it from: Firebase Console → Project Settings → Service Accounts\n"+
					"   Save the file as '%s' next to your main.go\n",
				credPath, credPath,
			)
		}
		log.Fatalf("❌ Firebase init failed: %v", err)
	}
	log.Println("✅ Firebase Admin SDK initialized")

	// ── Firestore ─────────────────────────────────────────────
	if err := FirestoreInitClient(projectID, credPath); err != nil {
		log.Fatalf("❌ Firestore init failed: %v", err)
	}
	log.Println("✅ Firestore initialized")
}

// ========================
// Built-in Handlers
// ========================

// FirebaseMeHandler is a ready-to-use handler that returns the authenticated
// user's profile (uid, email, permissions) extracted from the app-level JWT.
//
// Must be wrapped with FirebaseProtected middleware:
//
//	r.Get("/auth/me", grit.FirebaseProtected(secret)(grit.FirebaseMeHandler))
func FirebaseMeHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid, _ := ctx.Value(FirebaseUIDKey).(string)
	email, _ := ctx.Value(FirebaseEmailKey).(string)
	perms, _ := ctx.Value(FirebasePermissionsKey).([]string)

	respond(w, 200, true, "Authenticated user profile", map[string]interface{}{
		"uid":         uid,
		"email":       email,
		"permissions": perms,
	})
}

// ========================
// SIGNUP — Create user in Firebase, issue App JWT
// ========================

// FirebaseSignupHandler creates a new Firebase user with email + password,
// then issues a signed app-level JWT (same format as MongoDB/SQLite auth).
func FirebaseSignupHandler(jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			methodNotAllowed(w, r, "POST")
			return
		}

		if firebaseAuthClient == nil {
			respond(w, 500, false, "Firebase not initialized. Call grit.FirebaseInit() first.", nil)
			return
		}

		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respond(w, 400, false, "Invalid request body", nil)
			return
		}

		if body.Email == "" || body.Password == "" {
			respond(w, 400, false, "Email and password are required", nil)
			return
		}

		if len(body.Password) < 6 {
			respond(w, 400, false, "Password must be >= 6 characters", nil)
			return
		}

		// Create user in Firebase
		params := (&auth.UserToCreate{}).
			Email(body.Email).
			Password(body.Password).
			EmailVerified(false)

		firebaseUser, err := firebaseAuthClient.CreateUser(context.Background(), params)
		if err != nil {
			// Firebase returns a descriptive error — pass it on
			respond(w, 409, false, "Firebase error: "+err.Error(), nil)
			return
		}

		// Issue our own app-level JWT
		token := buildFirebaseJWT(firebaseUser.UID, firebaseUser.Email, jwtSecret)

		respond(w, 201, true, "Signup successful", map[string]interface{}{
			"token": token,
			"user": map[string]interface{}{
				"uid":   firebaseUser.UID,
				"email": firebaseUser.Email,
			},
		})
	}
}

// ========================
// SIGNIN — Verify Firebase ID Token, issue App JWT
// ========================

// FirebaseSigninHandler verifies a Firebase ID Token (obtained from the client SDK)
// and issues a signed app-level JWT.
//
// The client must first sign in using Firebase client SDK and send the ID Token:
//
//	POST /firebase/signin
//	{ "id_token": "<firebase_id_token>" }
func FirebaseSigninHandler(jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			methodNotAllowed(w, r, "POST")
			return
		}

		if firebaseAuthClient == nil {
			respond(w, 500, false, "Firebase not initialized. Call grit.FirebaseInit() first.", nil)
			return
		}

		var body struct {
			IDToken string `json:"id_token"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.IDToken == "" {
			respond(w, 400, false, "id_token is required", nil)
			return
		}

		// Verify Firebase ID Token server-side
		decoded, err := firebaseAuthClient.VerifyIDToken(context.Background(), body.IDToken)
		if err != nil {
			respond(w, 401, false, "Invalid or expired Firebase ID token", nil)
			return
		}

		// Issue our own app-level JWT
		token := buildFirebaseJWT(decoded.UID, decoded.Claims["email"].(string), jwtSecret)

		respond(w, 200, true, "Signin successful", map[string]interface{}{
			"token": token,
			"user": map[string]interface{}{
				"uid":   decoded.UID,
				"email": decoded.Claims["email"],
			},
		})
	}
}

// ========================
// FirebaseProtected Middleware
// ========================

// FirebaseProtected is JWT middleware for routes protected by Firebase-issued app tokens.
// It validates the app JWT and injects firebase_uid, firebase_email, firebase_permissions
// into the request context.
func FirebaseProtected(jwtSecret string) func(http.HandlerFunc) http.HandlerFunc {

	secret := []byte(jwtSecret)

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization required", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid token format. Use: Bearer <token>", http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {
				// Enforce HMAC — prevent algorithm confusion attacks
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return secret, nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "Invalid token claims", http.StatusUnauthorized)
				return
			}

			uid, _ := claims["uid"].(string)
			email, _ := claims["email"].(string)

			rawPerms, _ := claims["permissions"].([]interface{})
			perms := make([]string, 0)
			for _, p := range rawPerms {
				if s, ok := p.(string); ok {
					perms = append(perms, s)
				}
			}

			ctx := context.WithValue(r.Context(), FirebaseUIDKey, uid)
			ctx = context.WithValue(ctx, FirebaseEmailKey, email)
			ctx = context.WithValue(ctx, FirebasePermissionsKey, perms)

			next(w, r.WithContext(ctx))
		}
	}
}

// ========================
// JWT Builder (Firebase-specific)
// ========================

func buildFirebaseJWT(uid, email, secret string) string {
	claims := jwt.MapClaims{
		"uid":         uid,
		"email":       email,
		"permissions": []string{"user:read"},
		"exp":         time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(secret))
	return signed
}

// ========================
// SIGNOUT — Revoke Firebase Refresh Tokens
// ========================

// FirebaseSignoutHandler revokes all refresh tokens for the authenticated user
// in Firebase, effectively signing them out on all devices. The uid is read
// from the app-level JWT (set by FirebaseProtected middleware).
//
// Must be wrapped with FirebaseProtected middleware:
//
//	r.Post("/auth/signout", grit.FirebaseProtected(secret)(grit.FirebaseSignoutHandler))
func FirebaseSignoutHandler(w http.ResponseWriter, r *http.Request) {
	if firebaseAuthClient == nil {
		respond(w, 500, false, "Firebase not initialized. Call grit.FirebaseInit() first.", nil)
		return
	}

	uid, _ := r.Context().Value(FirebaseUIDKey).(string)
	if uid == "" {
		respond(w, 401, false, "Unauthorized: uid not found in token", nil)
		return
	}

	if err := firebaseAuthClient.RevokeRefreshTokens(context.Background(), uid); err != nil {
		respond(w, 500, false, "Failed to revoke tokens: "+err.Error(), nil)
		return
	}

	respond(w, 200, true, "Signed out successfully. All sessions revoked.", nil)
}
