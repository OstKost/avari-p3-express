package handler

import (
	"database/sql"
	"net/http"
	"time"

	_ "github.com/OstKost/avari-p3-express/apps/api/docs" // Swagger generated docs
	customMiddleware "github.com/OstKost/avari-p3-express/apps/api/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
)

// RouterConfig contains dependencies for building HTTP router.
type RouterConfig struct {
	P3Handler      *P3Handler
	WorkHandler    *WorkHandler
	DB             *sql.DB
	AllowedOrigins []string
}

// NewRouter builds and configures the Chi HTTP router.
func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	// Standard middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(customMiddleware.RequestLogger())
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(customMiddleware.CORS(cfg.AllowedOrigins))

	// Health check endpoint
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := cfg.DB.Ping(); err != nil {
			respondError(w, http.StatusServiceUnavailable, "Database unreachable")
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{
			"status":   "ok",
			"database": "connected",
			"time":     time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Swagger Documentation UI
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	if cfg.WorkHandler != nil {
		r.Post("/api/session", cfg.WorkHandler.Session)
		r.With(cfg.WorkHandler.Auth).Get("/api/session", func(w http.ResponseWriter, r *http.Request) {
			if !actor(r).Manager {
				respondError(w, 403, "Manager only")
				return
			}
			respondJSON(w, 200, map[string]string{"role": "manager"})
		})
	}

	// API v1 routes
	r.Route("/api/v1", func(api chi.Router) {
		if cfg.WorkHandler != nil {
			api.Use(cfg.WorkHandler.Auth)
			workRoutes(api, cfg.WorkHandler)
		}
		if cfg.P3Handler != nil {
			p3Routes(api, cfg.P3Handler)
		}
	})

	return r
}
