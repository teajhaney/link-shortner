package main

import (
	"context"
	"link-shortner/internal/handler"
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

	log.Println("using postgres store")
	srv := handler.NewServer("http://localhost:8080", pg)

	log.Println("listening on :8080")
	if err := http.ListenAndServe(":8080", srv.Routes()); err != nil {
		log.Fatal(err)
	}
}
