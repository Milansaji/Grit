package main

import (
	"time"

	"github.com/Milansaji/Grit/grit"
)

// model

type Todo struct {
	ID        uint      `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func main() {
	jwtSecret := "b7c98b2711103a901292e0dfb9f48339d05940d36b8ee6ea783c56f4b64dc1e1"

	r := grit.NewRouter()

	// model registration
	grit.RegisterModel("todo", &Todo{})

	r.Get("/health", grit.HealthHandler)

	// auth routes (SQLite)
	r.Post("/local/signup", grit.SignupSQLiteHandler)
	r.Post("/local/signin", grit.SigninSQLiteHandler(jwtSecret))
	r.Post("/local/signout", grit.ProtectSQLite(jwtSecret)(grit.SignoutSQLiteHandler))

	adminOnly := grit.RequirePermission(jwtSecret, "admin:all")
	userRead := grit.RequirePermission(jwtSecret, "user:read")

	// CRUD todo
	r.Post("/todo", adminOnly(grit.GritC("todo")))
	r.Get("/todos", userRead(grit.GritR("todo")))
	r.Get("/todo", userRead(grit.GritGetByID("todo")))
	r.Put("/todo", adminOnly(grit.GritU("todo")))
	r.Delete("/todo", adminOnly(grit.GritD("todo")))

	r.Start("8080")
}
