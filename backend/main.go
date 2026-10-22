package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"moviehub/backend/database"
	"moviehub/backend/handlers"
)

func main() {
	// เชื่อมต่อ PostgreSQL
	db, err := database.Connect()
	if err != nil {
		log.Fatal("Database connection failed: ", err)
	}
	defer db.Close()

	// สร้าง Auth Handler
	authHandler := &handlers.AuthHandler{
		DB: db,
	}

	mux := http.NewServeMux()

	// Health Check
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"message": "MovieHub Go Backend is running",
		})
	})

	// Register API
	mux.HandleFunc("POST /api/auth/register", authHandler.Register)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("MovieHub Backend running on port", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
