# Grit 🪨

**A zero-boilerplate Go backend framework.**  
Drop in auth, unified CRUD, AI/RAG, and middleware for SQLite, MongoDB, Firebase, or Supabase — in minutes, not hours.

---

## 💎 What's New in v2 (B.Tech Final Year Edition)
- **AI & RAG Support**: Built-in vector search and LLM integration using Ollama.
- **Unified CRUD API**: One set of handlers (`grit.C`, `grit.R`, etc.) for ALL 4 databases.
- **Persistent Auth**: SQLite signouts now survive server restarts (database-backed).
- **Database Agnosticity**: Switch between SQLite, Mongo, Firestore, and Supabase with ONE line of code.

---

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [AI & RAG (NEW!)](#ai--rag-new)
- [Unified CRUD Helpers](#unified-crud-helpers)
- [Switching Databases](#switching-databases)
- [Auth Providers](#auth-providers)
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

---

## AI & RAG (NEW!) 🚀

Grit now includes built-in support for **Retrieval-Augmented Generation** using local LLMs (via [Ollama](https://ollama.com/)).

### 1. Vector Store & Handlers
```go
// Initialize an in-memory vector store
store := grit.NewSimpleVectorStore()

// Register AI handlers
r.Post("/ingest", grit.RAGIngestHandler(store, "mxbai-embed-large"))
r.Post("/chat",   grit.RAGQueryHandler(store, "mistral", "mxbai-embed-large"))
```

### 2. Manual LLM Access
```go
// Get a direct response from LLM
response, err := grit.Prompt("mistral", "Why is Grit so fast?")

// Generate embeddings
vector, err := grit.Embed("mxbai-embed-large", "Grit 🪨")
```

> [!TIP]
> Ensure Ollama is running at `http://localhost:11434`. You can override this using `grit.SetLLMBaseURL("url")` or the `OLLAMA_HOST` environment variable.

---

## Unified CRUD Helpers

Grit provides a **Storage-Agnostic API**. You write the route once, and it works regardless of which database is plugged in.

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
r.Post("/auth/signup", grit.FirebaseSignupWithEmail(jwtSecret))
r.Post("/auth/signin", grit.FirebaseSigninWithEmail(jwtSecret))
```

### ⚡ Supabase Auth
```go
grit.SupabaseInit("https://xyz.supabase.co", "key")
r.Post("/auth/signup", grit.SupabaseSignup(jwtSecret))
r.Post("/auth/signin", grit.SupabaseSignin(jwtSecret))
```

---

## Project Structure

```
Grit/
├── grit/
│   ├── rag.go               # NEW: RAG & Vector Search
│   ├── llm.go               # NEW: Ollama/LLM Integration
│   ├── store.go             # Unified Store Interface (Mongo, SQL, Firestore, Supabase)
│   ├── grit.go              # Main public API surface
│   ├── router.go            # Testable Router implementation
├── firebase/
│   ├── examples/
│   │   ├── rag_server/      # AI Chatbot Example
└── README.md
```

---

## License
MIT — Final Year Project Edition.
