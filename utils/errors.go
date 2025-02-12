package utils

import (
	"log"
	"net/http"
)

// InternalServerError logs the error before sending a 500 response
func InternalServerError(w http.ResponseWriter) {
	log.Println("❌ Internal Server Error Occurred")
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
