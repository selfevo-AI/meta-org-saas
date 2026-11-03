package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

// MaxRequestBodyBytes caps every request body. Document import allows 20 MB of
// files plus multipart framing and the frontend dev proxy forwards up to 32 MB,
// so this is the ceiling; handlers apply tighter limits per route.
const MaxRequestBodyBytes = 32 << 20

func New(router *chi.Mux, port int) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		// Streaming handlers clear this per request via httpstream.
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
}

func NewRouter(corsOrigins []string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.LimitRequestBody(MaxRequestBodyBytes))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Organization-ID", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID", "X-Error-Code", "Retry-After"},
		AllowCredentials: true,
	}))
	return r
}
