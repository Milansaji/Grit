# Grit 🪨

**A zero-boilerplate Go backend framework.**  
Drop in auth, CRUD, and middleware for SQLite, MongoDB, Firebase, or Supabase — in minutes, not hours.

---

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Router](#router)
- [Auth Providers](#auth-providers)
  - [SQLite Auth](#-sqlite-auth)
  - [MongoDB Auth](#-mongodb-auth)
  - [Firebase Auth](#-firebase-auth)
  - [Supabase Auth](#-supabase-auth)
- [CRUD Helpers](#crud-helpers)
  - [SQLite / GORM (GritC/R/U/D)](#sqlite--gorm-gritcrud)
  - [MongoDB (MongoC/R/U/D)](#mongodb-mongocrud)
  - [Firestore (FirestoreC/R/U/D)](#firestore-firestorecrud)
  - [Supabase (SupabaseC/R/U/D)](#supabase-supabasecrud)
- [Middleware](#middleware)
  - [JWT Protection](#jwt-protection)
  - [Permission Guard](#permission-guard)
  - [Role Guard](#role-guard)
- [Built-in Features](#built-in-features)
- [Response Format](#response-format)
- [Complete Example — SQLite Todo App](#complete-example--sqlite-todo-app)
- [Complete Example — Firebase App](#complete-example--firebase-app)

---

## Installation

```bash
go get github.com/Milansaji/Grit
```

In your `go.mod`:

```
module your-app

go 1.21

require github.com/Milansaji/Grit v0.0.0
```

---

## Quick Start

```go
package main

import "github.com/Milansaji/Grit/grit"

func main() {
    r := grit.NewRouter()

    r.Get("/health", grit.HealthHandler)

    r.Start("8080")
}
```

Visit `http://localhost:8080/docs` for auto-generated Swagger UI. ✅

---

## Router

Grit includes a simple, zero-dependency router with built-in:

| Feature | Details |
|---------|---------|
| **CORS** | `Access-Control-Allow-*` headers set automatically |
| **Logging** | Colorized method + latency logs on every request |
| **Body Limit** | 1 MB request body limit enforced automatically |
| **Swagger UI** | Auto-generated at `/docs` and `/openapi.json` |
| **404 handler** | Returns clean JSON `{ "success": false }` |

### Methods

```go
r := grit.NewRouter()

r.Get("/path",    handler)
r.Post("/path",   handler)
r.Put("/path",    handler)
r.Patch("/path",  handler)
r.Delete("/path", handler)

r.Start("8080")
```

---

## Auth Providers

Grit supports **4 auth backends** with signup, signin, and signout — all producing app-level JWTs.

---

### 🗃️ SQLite Auth

Uses GORM + BCrypt. Auto-creates `auth.db`. First user registered automatically gets `admin:all` permission.

#### Init

No explicit init needed — auto-initializes on first request.

#### Routes

```go
jwtSecret := "your-secret"

r.Post("/auth/signup",  grit.SignupSQLiteHandler)
r.Post("/auth/signin",  grit.SigninSQLiteHandler(jwtSecret))
r.Post("/auth/signout", grit.ProtectSQLite(jwtSecret)(grit.SignoutSQLiteHandler))

// User lookups
r.Get("/auth/users", grit.GetAllUsersSQLiteHandler)
r.Get("/auth/user",  grit.GetUserByIDSQLiteHandler) // ?id=1
```

#### Requests

```jsonc
// POST /auth/signup
{ "email": "user@example.com", "password": "secret123" }

// POST /auth/signin
{ "email": "user@example.com", "password": "secret123" }

// POST /auth/signout
// Header: Authorization: Bearer <token>
```

#### Aliases

```go
grit.SignupSQLite          // → SignupSQLiteHandler
grit.SigninSQLite(secret)  // → SigninSQLiteHandler(secret)
```

#### Notes
- Signout uses an **in-memory token blacklist** — invalidated tokens are rejected by `ProtectSQLite` until server restart.
- Tokens expire in **24 hours**.

---

### 🍃 MongoDB Auth

Uses the official MongoDB driver + BCrypt. First user gets `admin:all` permission.

#### Init

```go
grit.MongoConnect("mongodb://localhost:27017", "mydb")
// or
grit.MongoInit("mongodb://localhost:27017", "mydb")
```

#### Routes

```go
r.Post("/auth/signup",  grit.SignupMongo(jwtSecret))
r.Post("/auth/signin",  grit.SigninMongo(jwtSecret))
r.Post("/auth/signout", grit.SignoutMongoHandler)

// User management
r.Get("/auth/users",   grit.GetAllUsersMongo())
r.Delete("/auth/user", grit.DeleteUserMongo())
```

#### Aliases

```go
grit.SignupMongo  // → CreateUserWithEmailAndPasswordMongo
grit.SigninMongo  // → SigninUserWithEmailAndPassMongo
```

---

### 🔥 Firebase Auth

Uses Firebase Admin SDK. Creates users in Firebase and issues app-level JWTs.

#### Init

```go
// Combined Firebase + Firestore init (recommended):
grit.InitFirebase("serviceAccountKey.json", "your-project-id")

// Firebase only:
grit.FirebaseInit("serviceAccountKey.json")
```

> Download `serviceAccountKey.json` from: **Firebase Console → Project Settings → Service Accounts**

#### Routes

```go
protect := grit.FirebaseProtected(jwtSecret)

r.Post("/auth/signup",  grit.FirebaseSignup(jwtSecret))
r.Post("/auth/signin",  grit.FirebaseSignin(jwtSecret))
r.Post("/auth/signout", protect(grit.FirebaseSignoutHandler))
r.Get("/auth/me",       protect(grit.FirebaseMeHandler))
```

#### Requests

```jsonc
// POST /auth/signup
{ "email": "user@example.com", "password": "secret123" }

// POST /auth/signin  — client sends Firebase ID Token from client SDK
{ "id_token": "<firebase_id_token>" }

// POST /auth/signout
// Header: Authorization: Bearer <app_jwt>
// Revokes ALL Firebase refresh tokens for the user (all devices)
```

#### Context Keys (inside protected handlers)

```go
uid   := r.Context().Value(grit.FirebaseUIDKey).(string)
email := r.Context().Value(grit.FirebaseEmailKey).(string)
perms := r.Context().Value(grit.FirebasePermissionsKey).([]string)
```

---

### ⚡ Supabase Auth

Uses Supabase REST API. Issues app-level JWTs after Supabase validates.

#### Init

```go
grit.SupabaseInit("https://xyzcompany.supabase.co", "your-anon-key")
// or
grit.SupabaseInitClient(url, key)
```

#### Routes

```go
r.Post("/auth/signup",  grit.SupabaseSignup(jwtSecret))
r.Post("/auth/signin",  grit.SupabaseSignin(jwtSecret))
r.Post("/auth/signout", grit.SupabaseSignoutHandler)
```

#### Requests

```jsonc
// POST /auth/signup
{ "email": "user@example.com", "password": "secret123" }

// POST /auth/signin
{ "email": "user@example.com", "password": "secret123" }

// POST /auth/signout
// Header: Authorization: Bearer <supabase_access_token>
// Calls Supabase /auth/v1/logout — invalidates server-side session
```

---

## CRUD Helpers

All CRUD functions work with `grit.RegisterModel()`.  
Register your model once — Grit handles the rest.

```go
type Post struct {
    ID    uint   `json:"id"`
    Title string `json:"title"`
    Body  string `json:"body"`
}

grit.RegisterModel("posts", &Post{})
```

---

### SQLite / GORM (`GritC/R/U/D`)

Auto-creates a `<name>.db` SQLite file per model.

| Function | Method | Description |
|----------|--------|-------------|
| `GritC(name)` | POST | Create a record |
| `GritR(name)` | GET | Fetch all records |
| `GritGetByID(name)` | GET/POST | Fetch by `id` (query param or body) |
| `GritU(name)` | PUT | Update by `id` in body |
| `GritD(name)` | DELETE | Delete by `id` in body |

```go
r.Post("/posts",  grit.GritC("posts"))
r.Get("/posts",   grit.GritR("posts"))
r.Get("/post",    grit.GritGetByID("posts"))  // GET /post?id=1
r.Put("/post",    grit.GritU("posts"))
r.Delete("/post", grit.GritD("posts"))
```

```jsonc
// POST /posts
{ "title": "Hello", "body": "World" }

// PUT /post
{ "id": 1, "title": "Updated" }

// DELETE /post
{ "id": 1 }
```

---

### MongoDB (`MongoC/R/U/D`)

| Function | Method | Description |
|----------|--------|-------------|
| `MongoC(name)` | POST | Insert document |
| `MongoR(name)` | GET | Fetch all documents |
| `MongoGetByID(name)` | GET | Fetch by `?id=<objectId>` |
| `MongoU(name)` | PUT | Update by `id` in body |
| `MongoD(name)` | DELETE | Delete by `id` in body |

```go
r.Post("/posts",  grit.MongoC("posts"))
r.Get("/posts",   grit.MongoR("posts"))
r.Get("/post",    grit.MongoGetByID("posts"))
r.Put("/post",    grit.MongoU("posts"))
r.Delete("/post", grit.MongoD("posts"))
```

---

### Firestore (`FirestoreC/R/U/D`)

| Function | Method | Description |
|----------|--------|-------------|
| `FirestoreC(name)` | POST | Add document |
| `FirestoreR(name)` | GET | Fetch all documents |
| `FirestoreGetByID(name)` | GET | Fetch by `?id=<docId>` |
| `FirestoreU(name)` | PUT/PATCH | Update by `id` in body |
| `FirestoreD(name)` | DELETE | Delete by `id` in body |
| `FirestoreWhere(name, field, op)` | GET | Filter by field, e.g. `?value=foo` |

```go
r.Post("/posts",           grit.FirestoreC("posts"))
r.Get("/posts",            grit.FirestoreR("posts"))
r.Get("/post",             grit.FirestoreGetByID("posts"))
r.Put("/post",             grit.FirestoreU("posts"))
r.Delete("/post",          grit.FirestoreD("posts"))
r.Get("/posts/by-author",  grit.FirestoreWhere("posts", "author", "=="))
```

---

### Supabase (`SupabaseC/R/U/D`)

| Function | Method | Description |
|----------|--------|-------------|
| `SupabaseC(name)` | POST | Insert row |
| `SupabaseR(name)` | GET | Fetch all rows (supports query params) |
| `SupabaseGetByID(name)` | GET | Fetch by `?id=<value>` |
| `SupabaseU(name)` | PUT/PATCH | Update by `id` in body |
| `SupabaseD(name)` | DELETE | Delete by `id` in body |

```go
r.Post("/posts",  grit.SupabaseC("posts"))
r.Get("/posts",   grit.SupabaseR("posts"))
r.Get("/post",    grit.SupabaseGetByID("posts"))
r.Put("/post",    grit.SupabaseU("posts"))
r.Delete("/post", grit.SupabaseD("posts"))
```

Supabase query params are **forwarded directly** — use any PostgREST filter:
```
GET /posts?order=created_at.desc&limit=10
```

---

## Middleware

### JWT Protection

```go
// SQLite JWT guard
protect := grit.ProtectSQLite(jwtSecret)
r.Get("/protected", protect(myHandler))

// Firebase JWT guard
protect := grit.FirebaseProtected(jwtSecret)
r.Get("/protected", protect(myHandler))
```

### Permission Guard

Checks the `permissions` array in the JWT. `admin:all` bypasses all permission checks.

```go
adminOnly := grit.RequirePermission(jwtSecret, "admin:all")
userRead  := grit.RequirePermission(jwtSecret, "user:read")

r.Post("/posts",  adminOnly(grit.GritC("posts")))  // admin only
r.Get("/posts",   userRead(grit.GritR("posts")))    // any logged-in user
```

> **Auto-permission logic:**  
> - First user to sign up → gets `admin:all`  
> - All subsequent users → get `user:read`

### Role Guard

Checks a `role` claim in the JWT.

```go
requireAdmin := grit.RequireRole(jwtSecret, "admin")
r.Delete("/user", requireAdmin(deleteHandler))

// Shorthand:
r.Delete("/user", grit.RequireAdmin(jwtSecret)(deleteHandler))
```

---

## Built-in Features

| Feature | How to use |
|---------|-----------|
| **Health check** | `r.Get("/health", grit.HealthHandler)` |
| **Swagger UI** | Auto-mounted at `/docs` on every router |
| **OpenAPI JSON** | Auto-mounted at `/openapi.json` |
| **CORS** | Applied globally — no config needed |
| **Request logging** | Colorized logs with method + latency |
| **Body size limit** | 1 MB enforced on all routes |

---

## Response Format

All Grit handlers return a consistent JSON envelope:

```json
{
  "success": true,
  "message": "Fetched successfully",
  "data": { ... },
  "meta": {
    "timestamp": "2026-03-02T16:30:00Z"
  }
}
```

Error responses:

```json
{
  "success": false,
  "message": "invalid credentials",
  "meta": {
    "timestamp": "2026-03-02T16:30:00Z"
  }
}
```

---

## Complete Example — SQLite Todo App

```go
package main

import (
    "time"
    "github.com/Milansaji/Grit/grit"
)

type Todo struct {
    ID        uint      `json:"id"`
    Title     string    `json:"title"`
    Body      string    `json:"body"`
    Done      bool      `json:"done"`
    Priority  string    `json:"priority"` // low | medium | high
    CreatedAt time.Time `json:"created_at"`
}

func main() {
    jwtSecret := "your-secret-key"

    grit.RegisterModel("todos", &Todo{})

    r := grit.NewRouter()

    protect   := grit.ProtectSQLite(jwtSecret)
    adminOnly := grit.RequirePermission(jwtSecret, "admin:all")
    userRead  := grit.RequirePermission(jwtSecret, "user:read")

    // Auth
    r.Post("/auth/signup",  grit.SignupSQLiteHandler)
    r.Post("/auth/signin",  grit.SigninSQLiteHandler(jwtSecret))
    r.Post("/auth/signout", protect(grit.SignoutSQLiteHandler))

    // Todos
    r.Post("/todos",   adminOnly(grit.GritC("todos")))
    r.Get("/todos",    userRead(grit.GritR("todos")))
    r.Get("/todo",     userRead(grit.GritGetByID("todos")))  // GET /todo?id=1
    r.Put("/todo",     adminOnly(grit.GritU("todos")))
    r.Delete("/todo",  adminOnly(grit.GritD("todos")))

    // Health
    r.Get("/health", grit.HealthHandler)

    r.Start("8080")
}
```

---

## Complete Example — Firebase App

```go
package main

import "github.com/Milansaji/Grit/grit"

type Post struct {
    Title  string `json:"title"`
    Body   string `json:"body"`
    Author string `json:"author"`
}

func main() {
    jwtSecret := "your-secret-key"

    grit.RegisterModel("posts", &Post{})

    // Init Firebase Admin SDK + Firestore
    grit.InitFirebase("serviceAccountKey.json", "your-project-id")

    r := grit.NewRouter()
    protect := grit.FirebaseProtected(jwtSecret)

    // Auth
    r.Post("/auth/signup",  grit.FirebaseSignup(jwtSecret))
    r.Post("/auth/signin",  grit.FirebaseSignin(jwtSecret))
    r.Post("/auth/signout", protect(grit.FirebaseSignoutHandler))
    r.Get("/auth/me",       protect(grit.FirebaseMeHandler))

    // Posts CRUD
    r.Post("/posts",  protect(grit.FirestoreC("posts")))
    r.Get("/posts",   protect(grit.FirestoreR("posts")))
    r.Get("/post",    protect(grit.FirestoreGetByID("posts")))
    r.Put("/post",    protect(grit.FirestoreU("posts")))
    r.Delete("/post", protect(grit.FirestoreD("posts")))

    // Filter by author
    r.Get("/posts/by-author", grit.FirestoreWhere("posts", "author", "=="))

    r.Get("/health", grit.HealthHandler)

    r.Start("8080")
}
```

---

## Project Structure

```
Grit/
├── grit/
│   ├── grit.go              # Aliases and public API surface
│   ├── router.go            # HTTP router, CORS, logging, body limit
│   ├── models.go            # RegisterModel, clone, makeSlice
│   ├── helpers.go           # respond(), HealthHandler, APIResponse
│   ├── auth_sqlite.go       # SQLite signup/signin/signout + JWT blacklist
│   ├── auth_mongo.go        # MongoDB signup/signin/signout
│   ├── firebase_auth.go     # Firebase Admin SDK auth + JWT
│   ├── supabase.go          # Supabase Auth REST + CRUD
│   ├── sqlite.go            # GritC/R/U/D (SQLite/GORM)
│   ├── mongo.go             # MongoC/R/U/D
│   ├── firestore.go         # FirestoreC/R/U/D/Where
│   ├── permission_middleware.go  # RequirePermission
│   ├── role_middleware.go        # RequireRole
│   ├── cors.go              # CORS middleware
│   ├── openapi.go           # OpenAPI spec generator
│   └── docs.go              # Swagger UI handler
├── firebase/
│   └── main.go              # Firebase example app
├── main.go                  # SQLite todo example
└── README.md
```

---

## License

MIT — free to use, modify, and distribute.
