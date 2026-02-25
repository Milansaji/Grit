# 🔥 Grit — Firebase + Firestore Example

This example demonstrates the **full Firebase feature set** of the Grit framework in a runnable Go server.

---

## Features Demonstrated

| Feature | Grit API | HTTP Route |
|---|---|---|
| Initialize Firebase Admin SDK | `grit.FirebaseInit(credPath)` | — |
| Initialize Firestore | `grit.FirestoreInit(projectID)` | — |
| User Signup | `grit.FirebaseSignup(secret)` | `POST /auth/signup` |
| User Signin (ID Token) | `grit.FirebaseSignin(secret)` | `POST /auth/signin` |
| Get current user profile | custom handler + context | `GET /auth/me` 🔒 |
| Signout (stateless) | custom handler | `POST /auth/signout` 🔒 |
| Protect a route (JWT middleware) | `grit.FirebaseProtected(secret)` | — |
| Read context values (UID, email, permissions) | `r.Context().Value(grit.FirebaseUIDKey)` | `GET /demo/context` 🔒 |
| Firestore Create | `grit.FirestoreC(collection)` | `POST /posts` 🔒 |
| Firestore Read All | `grit.FirestoreR(collection)` | `GET /posts` 🔒 |
| Firestore Read By ID | `grit.FirestoreGetByID(collection)` | `GET /post?id=...` 🔒 |
| Firestore Update (merge) | `grit.FirestoreU(collection)` | `PUT /post` 🔒 |
| Firestore Delete | `grit.FirestoreD(collection)` | `DELETE /post` 🔒 |
| Firestore Query by Field | `grit.FirestoreWhere(col, field, op)` | `GET /posts/by-author?value=...` 🔒 |
| Health check | custom handler | `GET /health` |

🔒 = Requires a valid `Authorization: Bearer <token>` header

---

## Setup

### Step 1 — Get your Service Account Key

1. Go to [Firebase Console](https://console.firebase.google.com)
2. Select your project → **Project Settings** → **Service Accounts**
3. Click **Generate new private key**
4. Save the downloaded file as `firebase/serviceAccountKey.json`

> ⚠️ Never commit `serviceAccountKey.json` to Git. It's already in `.gitignore`.

### Step 2 — Update `main.go` config

Open `firebase/main.go` and update the constants at the top:

```go
const (
    credPath  = "firebase/serviceAccountKey.json"  // path to your key
    projectID = "your-project-id"                  // Firebase project ID
    jwtSecret = "super-secret-jwt-key-change-me"   // use a long random string!
    collection = "posts"                            // Firestore collection name
    port      = "8080"
)
```

### Step 3 — Run the server

```bash
# From the Grit root directory:
go run firebase/main.go
```

### Step 4 — Open Swagger UI

```
http://localhost:8080/docs
```

---

## API Reference

### Auth

#### `POST /auth/signup`
Create a new Firebase user. Grit creates the user server-side via the Admin SDK.

```json
// Request body
{
  "email": "user@example.com",
  "password": "password123"
}

// Response
{
  "success": true,
  "message": "Signup successful",
  "data": {
    "token": "<app-level JWT>",
    "user": {
      "uid": "firebase-uid-here",
      "email": "user@example.com"
    }
  }
}
```

---

#### `POST /auth/signin`
Verify a Firebase ID Token obtained from the **Firebase client SDK** and receive an app-level JWT.

**Client-side (JavaScript) flow first:**
```javascript
import { signInWithEmailAndPassword, getAuth } from "firebase/auth";

const auth = getAuth();
const cred = await signInWithEmailAndPassword(auth, email, password);
const idToken = await cred.user.getIdToken();

// Then POST idToken to your backend:
const res = await fetch("http://localhost:8080/auth/signin", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ id_token: idToken })
});
const { data } = await res.json();
// data.token is your app JWT — store it and use it in Authorization headers
```

```json
// Request body
{
  "id_token": "<firebase_id_token_from_client_sdk>"
}

// Response
{
  "success": true,
  "message": "Signin successful",
  "data": {
    "token": "<app-level JWT>",
    "user": {
      "uid": "firebase-uid-here",
      "email": "user@example.com"
    }
  }
}
```

---

#### `GET /auth/me` 🔒
Returns the authenticated user's profile extracted from the JWT context.

```bash
curl -H "Authorization: Bearer <your_jwt>" http://localhost:8080/auth/me
```

---

### Firestore CRUD

All CRUD routes operate on the Firestore collection defined by `collection` (default: `"posts"`).

#### `POST /posts` 🔒 — Create
```json
// Body — any JSON object
{
  "title": "Hello, Grit!",
  "body": "This post was created via Firestore.",
  "author": "Milan",
  "status": "published"
}

// Response
{
  "success": true,
  "message": "Created successfully",
  "data": { "id": "<firestoreDocID>" }
}
```

#### `GET /posts` 🔒 — Read All
```bash
curl -H "Authorization: Bearer <token>" http://localhost:8080/posts
```
Returns all documents; each has an `"id"` field with the Firestore document ID.

#### `GET /post?id=<docID>` 🔒 — Read by ID
```bash
curl -H "Authorization: Bearer <token>" "http://localhost:8080/post?id=abc123"
```

#### `PUT /post` 🔒 — Update (Partial Merge)
Only the fields you include are updated — other fields are untouched.
```json
{
  "id":    "<firestoreDocID>",
  "title": "Updated Title"
}
```

#### `DELETE /post` 🔒 — Delete
```json
{ "id": "<firestoreDocID>" }
```

#### `GET /posts/by-author?value=Milan` 🔒 — Query by Field
Returns all posts where `author == "Milan"`.

#### `GET /posts/by-status?value=published` 🔒 — Query by Status
Returns all posts where `status == "published"`.

---

### Utility

#### `GET /health` — Health Check
```json
{ "status": "ok", "service": "grit-firebase-demo" }
```

#### `GET /demo/context` 🔒 — Context Values Demo
Shows how to read Firebase context values (UID, email, permissions) injected by the `FirebaseProtected` middleware.

---

## How JWT Protection Works

```
Client                          Grit Server
  │                                  │
  │── POST /auth/signup ────────────>│  Grit creates Firebase user (Admin SDK)
  │<─ { token: "<app JWT>" } ────────│  Issues app-level JWT (HS256)
  │                                  │
  │── GET /posts                     │
  │   Authorization: Bearer <JWT> ──>│  FirebaseProtected validates JWT
  │                                  │  Injects UID, Email, Permissions into context
  │<─ [ ...posts... ] ───────────────│  Handler reads Firestore, responds
```

The app-level JWT payload looks like this:
```json
{
  "uid":         "firebase-uid",
  "email":       "user@example.com",
  "permissions": ["user:read"],
  "exp":         1234567890
}
```

---

## Reading Context Values in Your Own Handlers

After `FirebaseProtected` middleware runs, inject context values are available in any downstream handler:

```go
func myHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    uid,   _ := ctx.Value(grit.FirebaseUIDKey).(string)
    email, _ := ctx.Value(grit.FirebaseEmailKey).(string)
    perms, _ := ctx.Value(grit.FirebasePermissionsKey).([]string)

    // Use uid, email, perms however you need
}
```

---

## File Structure

```
Grit/
├── grit/
│   ├── firebase_auth.go     ← FirebaseInit, FirebaseSignup, FirebaseSignin, FirebaseProtected
│   ├── firestore.go         ← FirestoreInit, FirestoreC/R/U/D, FirestoreGetByID, FirestoreWhere
│   └── ...
├── firebase/
│   ├── main.go              ← This example
│   ├── README.md            ← You are here
│   └── serviceAccountKey.json  ← Add yours here (gitignored)
└── go.mod
```
