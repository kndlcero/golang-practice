package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"web/routes"
	"web/sessions"
	"github.com/joho/godotenv"
	"github.com/jackc/pgx/v5" // PostgreSQL driver using pgx
)


func init() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️ Warning: No .env file found. Using environment variables.")
	} else {
		log.Println("✅ Loaded environment variables from .env file.")
	}
}

// runWebServer starts the web server
func runWebServer() {
	// Initialize PostgreSQL instead of Supabase API
	err := sessions.InitializeDB()
	if err != nil {
		log.Fatalf("❌ Failed to connect to PostgreSQL: %v\n", err)
	}
	log.Println("✅ Connected to Supabase PostgreSQL successfully")

	// Create router
	router := routes.NewRouter()
	if router == nil {
		log.Fatal("❌ Failed to initialize router.")
	}

	// Use PORT from environment variable (useful for deployment)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default to 8080 if no environment variable is set
	}

	fmt.Printf("🚀 Starting server on :%s...\n", port)
	err = http.ListenAndServe(":"+port, router)
	if err != nil {
		log.Fatalf("❌ Server crashed: %v", err)
	}
}

// testDBConnection verifies the PostgreSQL connection
func testDBConnection() {
	// Retrieve the database URL from the environment variable
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("❌ DATABASE_URL environment variable not set")
	}

	// Establish connection using pgx
	conn, err := pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect to the database: %v", err)
	}
	defer conn.Close(context.Background())

	// Test the connection by running a simple query
	var version string
	if err := conn.QueryRow(context.Background(), "SELECT version()").Scan(&version); err != nil {
		log.Fatalf("❌ Query failed: %v", err)
	}

	log.Println("✅ PostgreSQL connected successfully. Database version:", version)
}

func main() {
	// Test the database connection before starting the web server
	testDBConnection()

	// Run the web server
	runWebServer()
}
