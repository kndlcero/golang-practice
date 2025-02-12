package middleware

import (
	"log"
	"net/http"
	"web/sessions"
)

// AuthRequired middleware ensures user is authenticated
func AuthRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Retrieve session token from cookies
		cookie, err := r.Cookie("access_token")
		if err != nil {
			log.Println("⚠️ No access token found, redirecting to login")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		log.Printf("🔍 Found access token: %s", cookie.Value)

		// Validate token with Supabase
		user, err := sessions.GetCurrentUser(r)
		if err != nil || user == nil {
			log.Printf("⚠️ Invalid session, redirecting to login: %v", err)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		log.Printf("✅ Authenticated user: %s", user.Email) // Now works!

		// User is authenticated, proceed to the requested page
		next(w, r)
	}
}
