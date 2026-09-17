package main

import (
	"context"

	"link-shortner/internal/auth"
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

	// JWT_SECRET signs access tokens. It must be at least 32 bytes and must be
	// the same value on every instance, or tokens issued by one stop working
	// on the others. Never commit it; keep it in .env or the environment.
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is not set; generate one with `openssl rand -base64 32`")
	}

	jwtIssuer := os.Getenv("JWT_ISSUER")
	if jwtIssuer == "" {
		jwtIssuer = "link-shortner"
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

	// The JWT service is shared by the signin handler and the auth middleware,
	// so both halves agree on the same signing key and issuer.
	tokenService, err := auth.NewJWTService(jwtSecret, jwtIssuer)
	if err != nil {
		log.Fatalf("initializing jwt service: %v", err)
	}

	// The validator adds the revocation check on top of signing, so a token
	// that logout revoked stops being accepted instead of staying usable until
	// it expires.
	validator := auth.NewTokenValidator(tokenService, pg)
	refreshService := auth.NewRefreshService(tokenService, pg)
	signinService := auth.NewSigninService(pg, tokenService, refreshService)
	authHandler := auth.NewHandler(signinService, refreshService, validator)

	linkService := link.NewService("http://localhost:8080", pg)
	linkHandler := link.NewHandler(linkService)

	// The users package owns its route list, so the middleware is injected
	// here and applied there rather than the patterns being registered twice.
	userService := users.NewService(pg)
	userHandler := users.NewHandler(userService, auth.Middleware(validator))

	//routes
	router := server.New(
		authHandler,
		linkHandler,
		userHandler,
	)

	log.Println("listening on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}
