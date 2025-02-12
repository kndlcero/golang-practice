package utils

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
)

// ExecuteTemplate renders a template safely
func ExecuteTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	tmplPath := filepath.Join("templates", tmpl) // Correct template path
	t, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Printf("❌ Template error (%s): %v", tmpl, err)
		http.Error(w, "Template Not Found: "+tmpl, http.StatusInternalServerError)
		return
	}

	err = t.Execute(w, data)
	if err != nil {
		log.Printf("❌ Failed to execute template (%s): %v", tmpl, err)
		http.Error(w, "Template Execution Failed", http.StatusInternalServerError)
	}
}
