package main

import (
	"context"

	"link-shortner/internal/database"
	"link-shortner/internal/link"
	"link-shortner/internal/migrations"
	"link-shortner/internal/server"
	"link-shortner/internal/users"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Fatalf("loading .env: %v", err)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set; export it before starting the API")
	}

	ctx := context.Background()

	pg, err := database.NewPostgres(ctx, dsn)
	if err != nil {
		log.Fatalf("connecting to postgres: %v", err)
	}
	defer pg.Close()

	if err := database.Migrate(ctx, pg, migrations.Files); err != nil {
		log.Fatalf("migrating database: %v", err)
	}

	log.Println("Using postgres store")
	linkService := link.NewService("http://localhost:8080", pg)
	linkHandler := link.NewHandler(linkService)

	userService := users.NewService(pg)
	userHandler := users.NewHandler(userService)

	//routes
	router := server.New(
		linkHandler,
		userHandler,
	)

	log.Println("listening on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}
