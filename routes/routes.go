package routes

import (
	"log"
	"net/http"
	"web/middleware"
	"web/models"
	"web/sessions"
	"web/utils"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// NewRouter initializes routes
func NewRouter() *mux.Router {
	r := mux.NewRouter()

	// Public routes
	r.HandleFunc("/", middleware.AuthRequired(indexGetHandler)).Methods("GET")
	r.HandleFunc("/login", loginGetHandler).Methods("GET")
	r.HandleFunc("/login", loginPostHandler).Methods("POST")
	r.HandleFunc("/register", registerGetHandler).Methods("GET")
	r.HandleFunc("/register", registerPostHandler).Methods("POST")

	// Upload routes
	r.HandleFunc("/upload", middleware.AuthRequired(UploadPage)).Methods("GET")     // Show upload form
	r.HandleFunc("/upload", middleware.AuthRequired(UploadHandler)).Methods("POST") // Handle file upload

	// Protected routes (require authentication)
	r.HandleFunc("/profile", middleware.AuthRequired(profileGetHandler)).Methods("GET")
	r.HandleFunc("/logout", middleware.AuthRequired(logoutGetHandler)).Methods("GET")

	// Static files
	fs := http.FileServer(http.Dir("./static/"))
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))

	return r
}

// UploadPage serves the file upload form
func UploadPage(w http.ResponseWriter, r *http.Request) {
	// Check for success query parameter
	success := r.URL.Query().Get("success") == "true"

	// Prepare data to pass to the template
	data := struct {
		Success bool
	}{
		Success: success,
	}

	log.Println("✅ Upload page accessed")
	utils.ExecuteTemplate(w, "upload.html", data) // Pass the success flag to the template
}


// UploadHandler processes file uploads
func UploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Parse the form
	err := r.ParseMultipartForm(10 << 20) // Limit upload size to 10MB
	if err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	// Retrieve the file from the form data
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to retrieve file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Extract additional metadata fields from the form
	title := r.FormValue("title")
	description := r.FormValue("description")
	course := r.FormValue("course")
	college := r.FormValue("college")

	// Validate required fields
	if title == "" || description == "" || course == "" || college == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	// Upload file and save metadata
	user, err := sessions.GetCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := user.ID
	_, err = models.PostUpload(userID, header.Filename, title, description, course, college, file)
	if err != nil {
		log.Printf("❌ Upload failed: %v", err)
		http.Error(w, "Upload failed", http.StatusInternalServerError)
		return
	}

	log.Println("✅ File uploaded successfully")
	http.Redirect(w, r, "/upload?success=true", http.StatusSeeOther)
}

// indexGetHandler displays all uploads on the main page
func indexGetHandler(w http.ResponseWriter, r *http.Request) {
	var uploads []models.Upload

	// Fetch uploads along with the username (uploaded_by) from the database
	query := `
		SELECT 
			uploads.user_id, uploads.file_name, uploads.file_url, uploads.title,
			uploads.description, uploads.course, uploads.college, uploads.uploaded_at, 
			COALESCE(users.username, 'Unknown') AS uploaded_by
		FROM 
			uploads
		LEFT JOIN 
			users ON uploads.user_id = users.id
		ORDER BY 
			uploads.uploaded_at DESC
	`

	rows, err := sessions.DB.Query(query)
	if err != nil {
		log.Printf("❌ Error retrieving uploaded files: %v\n", err)
		utils.InternalServerError(w)
		return
	}
	defer rows.Close()

	// Iterate through query results and populate the uploads slice
	for rows.Next() {
		var upload models.Upload
		if err := rows.Scan(
			&upload.UserID, &upload.FileName, &upload.FileURL, &upload.Title,
			&upload.Description, &upload.Course, &upload.College, &upload.UploadedAt,
			&upload.UploadedBy,
		); err != nil {
			log.Printf("❌ Error scanning upload record: %v\n", err)
			utils.InternalServerError(w)
			return
		}
		uploads = append(uploads, upload)
	}

	// Render the uploads on the index page
	utils.ExecuteTemplate(w, "index.html", struct {
		Title       string
		Uploads     []models.Upload
		DisplayForm bool
	}{
		Title:       "All Uploads",
		Uploads:     uploads,
		DisplayForm: false,
	})
}


func loginGetHandler(w http.ResponseWriter, r *http.Request) {
	utils.ExecuteTemplate(w, "login.html", nil)
}

// profileGetHandler serves the profile page
func profileGetHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve the logged-in user from the session
	user, err := sessions.GetCurrentUser(r)
	if err != nil {
		log.Printf("❌ Failed to retrieve user: %v\n", err)
		utils.InternalServerError(w)
		return
	}

	// Render the profile template with user details
	utils.ExecuteTemplate(w, "profile.html", user)
}

func loginPostHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	email := r.PostForm.Get("email")
	password := r.PostForm.Get("password")

	log.Printf("🔍 Attempting login - Email: %s", email)

	// Authenticate user with PostgreSQL
	token, err := sessions.SignIn(email, password)
	if err != nil {
		log.Printf("❌ Login Failed: %v", err)
		utils.ExecuteTemplate(w, "login.html", "Invalid email or password")
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
	})

	log.Println("✅ Session cookie set. Redirecting to upload page")
	http.Redirect(w, r, "/", http.StatusFound)
}

func registerGetHandler(w http.ResponseWriter, r *http.Request) {
	utils.ExecuteTemplate(w, "register.html", nil)
}

func registerPostHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	email := r.PostForm.Get("email")
	password := r.PostForm.Get("password")
	username := r.PostForm.Get("username")

	log.Printf("🛠 Registering new user - Email: %s", email)

	// Register user in PostgreSQL
	err := sessions.SignUp(email, username, password)
	if err != nil {
		log.Printf("❌ Registration Error: %v\n", err)
		utils.ExecuteTemplate(w, "register.html", "Email already taken")
		return
	}

	log.Println("✅ Registration successful! Redirecting to login...")
	http.Redirect(w, r, "/login", http.StatusFound)
}

func logoutGetHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   "access_token",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}
