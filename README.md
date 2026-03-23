# Grit 🪨

**A zero-boilerplate Go backend framework.**  
Drop in auth, unified CRUD, and middleware for SQLite, MongoDB, Firebase, or Supabase — in minutes, not hours.

---

## 💎 What's New in v2 (B.Tech Final Year Edition)
- **Unified CRUD API**: One set of handlers (`grit.C`, `grit.R`, etc.) for ALL 4 databases.
- **Persistent Auth**: SQLite signouts now survive server restarts (database-backed).
- **Testable Architecture**: Refactored router and core logic with a full unit test suite.
- **Database Agnosticity**: Switch between SQLite, Mongo, Firestore, and Supabase with ONE line of code.

---

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Unified CRUD Helpers](#unified-crud-helpers-new)
- [Switching Databases](#switching-databases)
- [Auth Providers](#auth-providers)
  - [SQLite Auth](#-sqlite-auth)
  - [Firebase Auth](#-firebase-auth)
  - [Supabase Auth](#-supabase-auth)
- [Middleware](#middleware)
- [Complete Example](#complete-example)

---

## Installation

```bash
go get github.com/Milansaji/Grit/grit
```

---

## Quick Start

```go
package main

import "github.com/Milansaji/Grit/grit"

type Todo struct {
    ID    uint   `json:"id" gorm:"primaryKey"`
    Title string `json:"title"`
    Done  bool   `json:"done"`
}

func main() {
    // 1. Register your models
    grit.RegisterModel("todos", &Todo{})

    // 2. Pick your database (Default is sqlite)
    grit.SetStore("sqlite") // or "mongo", "firestore", "supabase"

    r := grit.NewRouter()

    // 3. One line CRUD!
    r.Post("/todos",   grit.C("todos"))        // Generic Create
    r.Get("/todos",    grit.R("todos"))        // Generic Read All
    r.Get("/todo",     grit.GetByID("todos"))  // Generic Read by ID (?id=1)
    r.Put("/todo",     grit.U("todos"))        // Generic Update
    r.Delete("/todo",  grit.D("todos"))        // Generic Delete

    r.Start("8080")
}
```

Visit `http://localhost:8080/docs` for auto-generated Swagger UI. ✅

---

## Unified CRUD Helpers (NEW!)

Grit now provides a **Storage-Agnostic API**. You write the route once, and it works regardless of which database is plugged in.

| Function | Method | Description |
|----------|--------|-------------|
| `grit.C(name)` | POST | Create a record |
| `grit.R(name)` | GET | Fetch all records |
| `grit.GetByID(name)` | GET | Fetch by `?id=<val>` |
| `grit.U(name)` | PUT/PATCH | Update by `id` in JSON body |
| `grit.D(name)` | DELETE | Delete by `id` (query or body) |

### Switching Databases
You can swap your entire infrastructure by changing one line in `main.go`.

```go
grit.SetStore("mongo")     // MongoDB Driver
grit.SetStore("firestore") // Firebase Admin SDK
grit.SetStore("supabase")  // Supabase REST API
grit.SetStore("sqlite")    // Local SQLite (Default)
```

---

## Auth Providers

### 🗃️ SQLite Auth
Uses GORM + BCrypt. First user is `admin:all`.  
**Feature**: Persistent Revocation — Signouts are stored in `auth.db`, so tokens remain invalid even if the server restarts.

```go
jwtSecret := "my-secret"
r.Post("/auth/signup",  grit.SignupSQLite)
r.Post("/auth/signin",  grit.SigninSQLite(jwtSecret))
r.Post("/auth/signout", grit.Protect(jwtSecret)(grit.SignoutSQLiteHandler))
```

### 🔥 Firebase Auth
```go
grit.InitFirebase("serviceAccountKey.json", "project-id")
r.Post("/auth/signup", grit.FirebaseSignup(jwtSecret))
```

### ⚡ Supabase Auth
```go
grit.SupabaseInit("https://xyz.supabase.co", "key")
r.Post("/auth/signup", grit.SupabaseSignup(jwtSecret))
```

---

## Project Structure

```
Grit/
├── grit/
│   ├── store.go             # NEW: Unified Store Interface & Engines
│   ├── grit.go              # Main public API surface
│   ├── router.go            # Testable Router implementation
│   ├── *_test.go            # Core unit test suite
│   ├── ...                  # DB-specific drivers
├── new_example.go           # Comprehensive unified example
├── firebase_example.go      # Firebase-specific unified example
└── README.md
```

---

## License
MIT — Final Year Project Edition.
