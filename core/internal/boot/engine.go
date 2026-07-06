package boot

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"

	"github.com/roidmc/quotagate/internal/config"
	kexerrors "github.com/roidmc/quotagate/internal/errors"
	"github.com/roidmc/quotagate/internal/middleware"
)

func InitEngine(cfg *config.Config) (*chi.Mux, error) {
	r := chi.NewRouter()

	r.Use(chimw.Recoverer)
	r.Use(corsConfig(cfg))
	r.Use(middleware.HTTPHeaders(cfg))

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		kexerrors.Abort(w, http.StatusNotFound, kexerrors.RouteNotFound())
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		kexerrors.Abort(w, http.StatusMethodNotAllowed, kexerrors.MethodNotAllowed())
	})

	return r, nil
}

// corsConfig returns a CORS middleware handler.
func corsConfig(cfg *config.Config) func(http.Handler) http.Handler {
	c := cors.New(cors.Options{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   cfg.CORS.AllowedMethods,
		AllowedHeaders:   cfg.CORS.AllowedHeaders,
		ExposedHeaders:   []string{"Content-Length"},
		AllowCredentials: cfg.CORS.AllowCredentials,
		MaxAge:           cfg.CORS.MaxAge,
	})

	return c.Handler
}
