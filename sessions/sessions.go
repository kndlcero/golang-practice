package sessions

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// DB is the global PostgreSQL connection
var DB *sql.DB

// JWT secret key (for signing tokens)
var jwtSecret = []byte("neknek1212") // Change this to a secure key!

// User struct (only includes essential fields)
type User struct {
	ID       string    `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

// InitializeDB connects to Supabase PostgreSQL
func InitializeDB() error {
	// Retrieve the DATABASE_URL from the environment
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("❌ DATABASE_URL environment variable not set")
	}

	// Connect to PostgreSQL using the DATABASE_URL
	var err error
	DB, err = sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("❌ Failed to open database connection: %v", err)
	}

	// Test the connection
	err = DB.Ping()
	if err != nil {
		return fmt.Errorf("❌ Failed to connect to PostgreSQL: %v", err)
	}

	log.Println("✅ Connected to PostgreSQL successfully")
	return nil
}

// SignUp registers a new user in PostgreSQL (Supabase DB)
func SignUp(email, username, password string) error {
	log.Printf("🔍 Checking if email %s already exists", email)
	email = strings.ToLower(email)
	// Check if email already exists
	var exists bool
	err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email=$1)", email).Scan(&exists)
	if err != nil {
		log.Printf("❌ Query error: %v", err)
		return err
	}
	if exists {
		log.Printf("❌ Email %s already taken", email)
		return errors.New("email already taken")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("❌ Error hashing password: %v", err)
		return err
	}

	log.Printf("🔒 Hashed password: %s", hashedPassword)

	// Insert user into PostgreSQL
	result, err := DB.Exec(
		"INSERT INTO users (email, username, password, created_at) VALUES ($1, $2, $3, NOW())",
		email, username, string(hashedPassword),
	)
	if err != nil {
		log.Printf("❌ Failed to insert user into users table: %v", err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("✅ User inserted successfully. Rows affected: %d", rowsAffected)
	return nil
}

// SignIn authenticates a user and returns a JWT token
func SignIn(email, password string) (string, error) {
	var user User
	var hashedPassword string

	log.Printf("🔍 Checking database for email: %s", email)

	// Retrieve user from PostgreSQL
	err := DB.QueryRow("SELECT id, email, username, password FROM users WHERE email=$1", email).
		Scan(&user.ID, &user.Email, &user.Username, &hashedPassword)
	if err != nil {
		log.Printf("❌ User not found for email: %s. Error: %v", email, err)
		return "", errors.New("invalid email or password")
	}

	// Compare passwords
	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		log.Printf("❌ Password mismatch for email: %s", email)
		return "", errors.New("invalid email or password")
	}

	log.Printf("✅ User found: ID=%s, Email=%s", user.ID, user.Email)

	// Generate JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"exp":     time.Now().Add(time.Hour * 24).Unix(), // Token expires in 24 hours
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		log.Printf("❌ Error generating token: %v", err)
		return "", err
	}

	log.Println("✅ Login successful, JWT token generated.")
	return tokenString, nil
}

// ParseJWT parses a JWT token and returns user claims
func ParseJWT(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// GetCurrentUser retrieves the logged-in user from the request
func GetCurrentUser(r *http.Request) (*User, error) {
	// Get JWT token from cookie
	cookie, err := r.Cookie("access_token")
	if err != nil {
		log.Println("⚠️ No access token found in request")
		return nil, errors.New("unauthorized")
	}

	// Parse JWT token
	claims, err := ParseJWT(cookie.Value)
	if err != nil {
		log.Println("❌ Invalid token:", err)
		return nil, errors.New("unauthorized")
	}

	// Retrieve user from database
	var user User
	err = DB.QueryRow("SELECT id, email, username FROM users WHERE id=$1", claims["user_id"]).Scan(&user.ID, &user.Email, &user.Username)
	if err != nil {
		return nil, errors.New("user not found")
	}

	return &user, nil
}

// HTTPClient is a global HTTP client (can be reused for all requests)
var HTTPClient = &http.Client{}

// CreateStorageUploadRequest generates an HTTP POST request for Supabase Storage
func CreateStorageUploadRequest(filePath string, file *os.File) (*http.Request, error) {
	SUPABASE_URL := os.Getenv("SUPABASE_URL")
	SUPABASE_BUCKET_NAME := os.Getenv("SUPABASE_BUCKET_NAME")
	SUPABASE_KEY := os.Getenv("SUPABASE_KEY") // Replace with your actual Supabase service role key

	// Prepare the URL
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", SUPABASE_URL, SUPABASE_BUCKET_NAME, filePath)

	// Create a multipart form data buffer
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Add the file part
	part, err := writer.CreateFormFile("file", file.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %v", err)
	}

	_, err = file.Seek(0, 0) // Reset file pointer to the beginning
	if err != nil {
		return nil, fmt.Errorf("failed to reset file pointer: %v", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, fmt.Errorf("failed to copy file content: %v", err)
	}

	// Close the writer to finalize the form
	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close writer: %v", err)
	}

	// Create the HTTP POST request
	req, err := http.NewRequest("POST", url, &requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Add headers
	req.Header.Set("Authorization", "Bearer "+SUPABASE_KEY)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	return req, nil
}

// SignOut clears the user's session by deleting the cookie
func SignOut(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   "access_token",
		Value:  "",
		Path:   "/",
		MaxAge: -1, // Expire immediately
	})
}
