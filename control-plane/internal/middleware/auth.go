package middleware

import "net/http"

// Auth is a placeholder authentication middleware.
// TODO: implement JWT/API key validation.
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
