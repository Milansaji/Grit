package main

import (
	"log"
	"os"
	"time"

	"github.com/Milansaji/Grit/grit"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Read env variables
	jwtSecret := os.Getenv("JWT_SECRET")
	mongoURI := os.Getenv("MONGO_URI")

	if jwtSecret == "" || mongoURI == "" {
		log.Fatal("Missing JWT_SECRET or MONGO_URI in .env")
	}

	grit.MongoConnect(
		mongoURI,
		"gritdb",
	)

	r := grit.NewRouter()
	r.Post("/signup", grit.SignupMongo(jwtSecret))
	r.Post("/signin", grit.SigninMongo(jwtSecret))
	r.Post("/local/signup", grit.SignupSQLite)
	r.Post("/local/signin", grit.SigninSQLite(jwtSecret))

	auth := grit.MongoProtected(jwtSecret)

	r.Get("/users", auth(grit.RequirePermission(jwtSecret, "admin:all")(grit.GetAllUsersMongo())))

	r.Delete("/user", auth(grit.RequirePermission(jwtSecret, "admin:all")(grit.DeleteUserMongo())))
	grit.RegisterModel("blogs", &Blog{})
	r.Post("/blogs", auth(grit.RequirePermission(jwtSecret, "user:read")(grit.MongoC("blogs"))))
	r.Get("/blogs", auth(grit.RequirePermission(jwtSecret, "user:read")(grit.MongoR("blogs"))))
	r.Get("/blog", auth(grit.RequirePermission(jwtSecret, "user:read")(grit.MongoGetByID("blogs"))))
	r.Put("/blog", auth(grit.RequirePermission(jwtSecret, "user:read")(grit.MongoU("blogs"))))
	r.Delete("/blog", auth(grit.RequirePermission(jwtSecret, "admin:all")(grit.MongoD("blogs"))))

	r.Get("/local/blogs", grit.GritR("blogs"))
	r.Get("/local/blog", grit.GritGetByID("blogs"))
	r.Post("/local/blogs", grit.GritC("blogs"))
	r.Put("/local/blogs", grit.GritU("blogs"))
	r.Delete("/local/blogs", grit.GritD("blogs"))

	r.Start("8081")
}

type Blog struct {
	ID primitive.ObjectID `bson:"_id,omitempty" json:"id"`

	Title       string `bson:"title" json:"title"`
	Slug        string `bson:"slug" json:"slug"`
	Content     string `bson:"content" json:"content"`
	Thumbnail   string `bson:"thumbnail" json:"thumbnail"`
	IsPublished bool   `bson:"is_published" json:"is_published"`

	// Owner (JWT user)
	UserID primitive.ObjectID `bson:"user_id" json:"user_id"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}
