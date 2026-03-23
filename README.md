# Grit 🪨

**A zero-boilerplate Go backend framework.**  
Drop in auth, unified CRUD, AI/RAG, and middleware for SQLite, MongoDB, Firebase, or Supabase — in minutes, not hours.

---

## 💎 What's New in v2 (B.Tech Final Year Edition)
- **AI & RAG Support**: Built-in vector search and LLM integration using Ollama.
- **AI Agentic CRUD**: Control your entire data layer using natural language prompts.
- **Profile-Bound Agent Actions**: AI post create/update/delete is now tied to authenticated user profile.
- **Ownership Guardrails**: Users can edit/delete only their own posts (server-enforced).
- **Agent UX Upgrades**: Quick commands, model-aware action logs, and real error reporting in the web UI.
- **Unified CRUD API**: One set of handlers (`grit.C`, `grit.R`, etc.) for ALL 4 databases.
- **Persistent Auth**: SQLite signouts now survive server restarts (database-backed).
- **Database Agnosticity**: Switch between SQLite, Mongo, Firestore, and Supabase with ONE line of code.

---

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [AI Agentic CRUD (NEW!)](#ai-agentic-crud-new)
- [Agentic Web App (Firebase Example)](#agentic-web-app-firebase-example)
- [AI & RAG](#ai--rag)
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

## AI Agentic CRUD (NEW!) 🤖

Grit now allows you to control your entire backend using natural language through the **AI Agent**. It automatically discovers your registered models and translates user prompts into database actions.

### Usage
```go
// 1. Register your models as usual
grit.RegisterModel("tasks", &Task{})

// 2. Add the AI Agent route
r.Post("/agent", protect(grit.AICrud("mistral")))
```

### Supported Actions
The agent maps prompts to these actions:
- `create`
- `read_all`
- `read_by_id`
- `update`
- `delete`

Action/model names are normalized (for example: `edit` → `update`, `remove` → `delete`).

### Guardrails (Recommended)
For production-style behavior, protect your agent route and enforce ownership checks in handlers:
- Require JWT auth on `/agent`
- Bind `author` from JWT profile for post creation
- Allow update/delete only for posts owned by current user

### Good Prompt Examples
- `Create a new post titled "My Day 1" with body "Started building with Grit" and status "draft"`
- `Show my posts`
- `Edit my post titled "My Day 1" and set body to "Updated story body"`
- `Update my post titled "My Day 1" set status to "published"`
- `Delete my post titled "My Day 1"`
- `Delete my post with id "<POST_ID>"`

> [!TIP]
> If titles are duplicated, pass explicit `id` for update/delete.

### Example Prompts
- `"Create a task to buy milk"`
- `"Show me all pending tasks"`
- `"Delete the task with id 5"`
- `"Update task 1 to be done"`

> [!NOTE]
> The AI Agent uses the same **Unified Store** under the hood, so it works seamlessly across SQLite, MongoDB, Firestore, and Supabase.

---

## Agentic Web App (Firebase Example)

The Firebase example now includes a profile-aware agentic flow:

- **Protected AI endpoint**: `POST /agent`
- **Prompt examples endpoint**: `GET /agent/examples`
- **My posts endpoint**: `GET /posts/mine`
- **Ownership-safe mutations**:
    - `POST /posts/create` (author auto-bound from JWT)
    - `PUT/PATCH /post` (edit own post only)
    - `DELETE /post` (delete own post only)

### Firebase Boot
```go
grit.InitFirebase("serviceAccountKey.json", "project-id", "firebase-web-api-key")
```

### Agent Response Shape
```json
{
    "success": true,
    "message": "Action executed successfully",
    "data": {
        "action": { "action": "update", "model": "posts", "id": "...", "data": {"status":"published"} },
        "result": "updated successfully"
    }
}
```

On failure, `success=false` with proper HTTP error status and message.

---

## AI & RAG 🚀

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
grit.InitFirebase("serviceAccountKey.json", "project-id", "firebase-web-api-key")
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
│   ├── agent.go             # NEW: Agentic CRUD + intent normalization + ownership logic
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
