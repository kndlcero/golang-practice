package models

import (
	"database/sql"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
	"web/sessions"
)

// Upload represents a file upload with additional metadata
type Upload struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	FileName    string    `json:"file_name"`
	FileURL     string    `json:"file_url"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Course      string    `json:"course"`
	College     string    `json:"college"`
	UploadedBy  string    `json:"uploaded_by"` // User's name
	UploadedAt  time.Time `json:"uploaded_at"`
}

// PostUpload creates a new upload entry in the database and uploads the file to storage
func PostUpload(userID, fileName, title, description, course, college string, file multipart.File) (*Upload, error) {
	// Retrieve the name of the user based on the userID
	var uploadedBy string
	err := sessions.DB.QueryRow(`SELECT username FROM users WHERE id = $1`, userID).Scan(&uploadedBy)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user with ID %s not found", userID)
		}
		return nil, fmt.Errorf("failed to retrieve user name: %v", err)
	}

	// Generate a unique file name for storage
	uniqueFileName := fmt.Sprintf("%d_%s", time.Now().Unix(), fileName)
	filePath := fmt.Sprintf("uploads/%s", uniqueFileName)

	// Upload the file to storage
	err = uploadFileToStorage(filePath, file)
	if err != nil {
		return nil, fmt.Errorf("file upload failed: %v", err)
	}

	// Construct the public file URL
	SUPABASE_URL := os.Getenv("SUPABASE_URL")
	SUPABASE_BUCKET_NAME := os.Getenv("SUPABASE_BUCKET_NAME")
	fileURL := fmt.Sprintf("%s/storage/v1/object/public/%s/%s", SUPABASE_URL, SUPABASE_BUCKET_NAME, filePath)

	// Insert the upload metadata into the database
	upload := Upload{
		UserID:      userID,
		FileName:    fileName,
		FileURL:     fileURL,
		Title:       title,
		Description: description,
		Course:      course,
		College:     college,
		UploadedBy:  uploadedBy, // Set the name of the uploader
		UploadedAt:  time.Now(),
	}
	_, err = sessions.DB.Exec(
		`INSERT INTO uploads (user_id, file_name, file_url, title, description, course, college, uploaded_at) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		upload.UserID, upload.FileName, upload.FileURL, upload.Title, upload.Description, upload.Course, upload.College, upload.UploadedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to save upload metadata: %v", err)
	}

	return &upload, nil
}

// uploadFileToStorage uploads the file to storage
func uploadFileToStorage(filePath string, file multipart.File) error {
	// Create a temporary file
	tempFile, err := os.CreateTemp("", "upload-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Copy file content to the temporary file
	_, err = io.Copy(tempFile, file)
	if err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}

	// Reset the file pointer to the beginning
	if _, err := tempFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to reset file pointer: %v", err)
	}

	// Create the upload request
	req, err := sessions.CreateStorageUploadRequest(filePath, tempFile)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %v", err)
	}

	// Execute the request
	resp, err := sessions.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("storage upload request failed: %v", err)
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("storage upload failed with status: %d", resp.StatusCode)
	}

	return nil
}
