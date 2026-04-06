package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"url-shortening-service/internal/cache"
	"url-shortening-service/internal/handlers"
	"url-shortening-service/internal/store"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// connect to postgres
	s, err := store.New(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// connect to cache
	c := cache.New(os.Getenv("REDIS_URL"))
	log.Println("Cache ready")

	// get handlers set up
	handlers := handlers.New(s, c)

	// make router
	router := http.NewServeMux()

	router.HandleFunc("/api/health", handlers.Health)
	router.HandleFunc("/shorten", handlers.Shorten)
	router.HandleFunc("/shorten/", handlers.Shorten)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server running on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, handlers.CORS(router)))
}
