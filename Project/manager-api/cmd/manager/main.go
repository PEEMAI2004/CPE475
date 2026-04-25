package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"github.com/potbuddy/manager-api/internal/api"
	"github.com/potbuddy/manager-api/internal/ca"
	"github.com/rs/cors"
)

func main() {
	jwtSecret := []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		jwtSecret = []byte("dev-secret-keep-it-safe")
	}

	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")

	var caInstance *ca.CA
	if os.Getenv("ENABLE_CA") == "true" || os.Getenv("ENABLE_CA") == "1" {
		var err error
		caInstance, err = ca.LoadOrCreateCA("certs/ca.crt", "certs/ca.key")
		if err != nil {
			log.Fatalf("Failed to initialize CA: %v", err)
		}
		log.Println("CA initialized successfully.")
	} else {
		log.Println("CA is disabled. mTLS certificate signing will be unavailable.")
	}

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@postgresql.iot.kaminjitt.com:5432/potbuddy?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	server := api.NewServer(db, caInstance, jwtSecret, googleClientID)

	// CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	})
	handler := c.Handler(server.Router)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("Manager API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
