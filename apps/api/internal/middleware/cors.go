package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// CORS configures and returns standard CORS middleware.
func CORS(allowedOrigins []string) func(next http.Handler) http.Handler {
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}

	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Request-ID", "X-Avari-Local"},
		ExposedHeaders:   []string{"Link", "Content-Length", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300, // 5 minutes
	})
}
