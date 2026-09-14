package main

import (
	"context"
	"link-shortner/internal/link"
	"link-shortner/internal/server"
	"link-shortner/internal/store"
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

	pg, err := store.NewPostgres(context.Background(), dsn)
	if err != nil {
		log.Fatalf("connecting to postgres: %v", err)
	}
	defer pg.Close()

	log.Println("Using postgres store")
	linkService := link.NewService("http://localhost:8080", pg)
	linkHandler := link.NewHandler(linkService)

	log.Println("listening on :8080")
	if err := http.ListenAndServe(":8080", server.New(linkHandler)); err != nil {
		log.Fatal(err)
	}
}
