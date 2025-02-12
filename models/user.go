package models

import (
	"errors"
	"log"
	"web/sessions"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrInvalidLogin = errors.New("invalid login")
	ErrEmailTaken   = errors.New("email already taken")
)

// User struct represents a user in the database
type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"-"`
}

// NewUser creates a new user in Supabase PostgreSQL
func NewUser(email, username, password string) (*User, error) {
	// Check if email already exists
	var exists bool
	err := sessions.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email=$1)", email).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailTaken
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Insert user into PostgreSQL (Supabase DB)
	stmt, err := sessions.DB.Prepare("INSERT INTO users (email, username, password) VALUES ($1, $2, $3) RETURNING id")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var userID string
	err = stmt.QueryRow(email, username, string(hashedPassword)).Scan(&userID)
	if err != nil {
		return nil, err
	}

	log.Printf("✅ User created successfully - ID: %s, Email: %s, Username: %s", userID, email, username)

	return &User{
		ID:       userID,
		Email:    email,
		Username: username,
	}, nil
}

// GetUserByEmail retrieves a user by email from PostgreSQL
func GetUserByEmail(email string) (*User, error) {
	var user User
	err := sessions.DB.QueryRow("SELECT id, email, username, password FROM users WHERE email=$1", email).
		Scan(&user.ID, &user.Email, &user.Username, &user.Password)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return &user, nil
}

// AuthenticateUser verifies a user's email and password
func AuthenticateUser(email, password string) (*User, error) {
	log.Printf("🔍 Attempting login - Email: %s", email)

	// Retrieve user from database
	user, err := GetUserByEmail(email)
	if err != nil {
		log.Printf("❌ Login failed: user not found")
		return nil, ErrInvalidLogin
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		log.Printf("❌ Login failed: invalid password")
		return nil, ErrInvalidLogin
	}

	return user, nil
}

// GetUserById retrieves a user by ID from PostgreSQL
func GetUserById(id int) (*User, error) {
	var user User
	err := sessions.DB.QueryRow("SELECT id, email, username FROM users WHERE id=$1", id).
		Scan(&user.ID, &user.Email, &user.Username)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return &user, nil
}

// RegisterUser creates a new user
func RegisterUser(email, username, password string) error {
	log.Printf("🛠 Registering user - Email: %s, Username: %s", email, username)
	_, err := NewUser(email, username, password)
	if err != nil {
		log.Printf("❌ Registration Error: %v", err)
	}
	return err
}
