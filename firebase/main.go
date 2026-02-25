// ============================================================
//  🔥 Grit — Firebase + Firestore Example (Model-Aware)
// ============================================================
//
//  HOW TO RUN
//  ──────────
//  1. Place serviceAccountKey.json in this (firebase/) folder.
//  2. Update the config constants below.
//  3. Run from the firebase/ directory:
//       go run main.go
//  4. Open Swagger UI:
//       http://localhost:3001/docs
//
// ============================================================

package main

import (
	"fmt"
	"log"

	"github.com/milansaji/grit/grit"
)

// ============================================================
//  ⚙️  CONFIG
// ============================================================

const (
	credPath  = "serviceAccountKey.json"
	projectID = "cem-coxdo"
	jwtSecret = "b7c98b2711103a901292e0dfb9f48339d05940d36b8ee6ea783c56f4b64dc1e1"
	port      = "3001"
)

// ============================================================
//  📦  MODELS
// ============================================================

// Post is the model for the "posts" Firestore collection.
type Post struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Author string `json:"author"`
	Status string `json:"status"` // "draft" | "published"
}

// Comment is the model for the "comments" Firestore collection.
type Comment struct {
	PostID  string `json:"post_id"`
	Author  string `json:"author"`
	Content string `json:"content"`
}

// ============================================================
//  🚀  MAIN
// ============================================================

func main() {

	// ── 1. Register models ─────────────────────────────────────
	//  Same pattern as MongoDB / SQLite.
	//  After this, all Firestore CRUD functions know what
	//  "posts" and "comments" look like.
	grit.RegisterModel("posts", &Post{})
	grit.RegisterModel("comments", &Comment{})

	// ── 2. Initialize Firebase + Firestore ─────────────────────
	//  One call handles everything:
	//    • Firebase Admin SDK init
	//    • Firestore client init
	//    • Error checking with friendly messages
	//    • Logging on success
	//    • log.Fatal on any failure (server never starts broken)
	grit.InitFirebase(credPath, projectID)

	// ── 3. Build router ────────────────────────────────────────
	r := grit.NewRouter()
	protect := grit.FirebaseProtected(jwtSecret)

	// ── 4. Auth routes ─────────────────────────────────────────
	r.Post("/auth/signup", grit.FirebaseSignup(jwtSecret))
	r.Post("/auth/signin", grit.FirebaseSignin(jwtSecret))

	// FirebaseMeHandler is built into the grit framework —
	// no custom handler needed.
	r.Get("/auth/me", protect(grit.FirebaseMeHandler))

	// ── 5. Posts CRUD ──────────────────────────────────────────
	r.Post("/posts/create", protect(grit.FirestoreC("posts")))
	r.Get("/posts", protect(grit.FirestoreR("posts")))
	r.Get("/post", protect(grit.FirestoreGetByID("posts")))
	r.Put("/post", protect(grit.FirestoreU("posts")))
	r.Patch("/post", protect(grit.FirestoreU("posts")))
	r.Delete("/post", protect(grit.FirestoreD("posts")))

	// ── 6. Comments CRUD ───────────────────────────────────────
	r.Post("/comments", protect(grit.FirestoreC("comments")))
	r.Get("/comments", protect(grit.FirestoreR("comments")))
	r.Get("/comment", protect(grit.FirestoreGetByID("comments")))
	r.Put("/comment", protect(grit.FirestoreU("comments")))
	r.Delete("/comment", protect(grit.FirestoreD("comments")))

	// ── 7. Query routes ────────────────────────────────────────
	r.Get("/posts/by-author", protect(grit.FirestoreWhere("posts", "author", "==")))
	r.Get("/posts/published", protect(grit.FirestoreWhere("posts", "status", "==")))
	r.Get("/comments/by-post", protect(grit.FirestoreWhere("comments", "post_id", "==")))

	// ── 8. Health check — built into grit, no custom handler ───
	r.Get("/health", grit.HealthHandler)

	// ── 9. Start ───────────────────────────────────────────────
	fmt.Printf("\n🔥 Firebase demo → http://localhost:%s\n", port)
	fmt.Printf("📖 Swagger docs  → http://localhost:%s/docs\n\n", port)

	if err := r.Start(port); err != nil {
		log.Fatal(err)
	}
}
