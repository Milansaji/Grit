package main

import "github.com/Milansaji/Grit.git/grit"

func main() {

	// Mongo init
	grit.MongoConnect(
		"mongodb+srv://larkmaintenance_db_user:h2JQ40SheIlxYVsK@cluster0.ud2gne2.mongodb.net/?appName=Cluster0",
		"gritdb",
	)

	r := grit.NewRouter()

	// Auth
	r.Post("/signup", grit.SignupMongo("b7c98b2711103a901292e0dfb9f48339d05940d36b8ee6ea783c56f4b64dc1e1"))
	r.Post("/signin", grit.SigninMongo("b7c98b2711103a901292e0dfb9f48339d05940d36b8ee6ea783c56f4b64dc1e1"))

	// Protected route
	r.Get("/users",
		grit.Protect("b7c98b2711103a901292e0dfb9f48339d05940d36b8ee6ea783c56f4b64dc1e1")(
			grit.GetAllUsersMongo(),
		),
	)

	grit.RegisterModel("products", &Product{})

	r.Post("/products", grit.GritC("products"))
	r.Get("/products", grit.GritR("products"))
	r.Post("/product", grit.GritGetByID("products"))
	r.Put("/product", grit.GritU("products"))
	r.Delete("/product", grit.GritD("products"))

	r.Start("8080")
}

type Product struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price"`
}
