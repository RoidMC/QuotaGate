// QuotaGate universal router setup, customized and modified based on KexCore IAM project

package router

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/roidmc/quotagate/internal/authz"
	"github.com/roidmc/quotagate/internal/handler"
	"github.com/roidmc/quotagate/internal/middleware"
	"github.com/roidmc/quotagate/internal/service"
	"github.com/roidmc/quotagate/pkg/kexswiftdb"
)

type HandlerSet struct {
	Auth    *handler.AuthHandler
	Account *handler.AccountHandler
}

func Setup(r *chi.Mux, handlers *HandlerSet, tokenIssuer *service.TokenIssuer, authzManager *authz.AuthzManager, store kexswiftdb.Store, storeDriver string) {
	// Global security middleware: applied to ALL routes.
	r.Use(middleware.LimitRequestURI(middleware.DefaultMaxRequestURI))
	r.Use(middleware.LimitRequestBody(middleware.DefaultMaxRequestBody))

	r.Get("/", healthCheck(store))
	r.Get("/health", healthCheck(store))
	r.Get("/debug/store", storeDebugHandler(store, storeDriver))

	r.Route("/api", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", handlers.Auth.Register)
			r.Post("/login", handlers.Auth.Login)
			r.Post("/refresh", handlers.Auth.RefreshToken)
			r.Get("/methods", handlers.Auth.Methods)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.BearerAuth(tokenIssuer))
			r.Use(middleware.Authz(authzManager))

			r.Route("/my-account", func(r chi.Router) {
				r.Get("/", handlers.Account.GetMyAccount)
				r.Put("/", handlers.Account.UpdateMyAccount)
				r.Post("/password", handlers.Account.ChangePassword)
			})

			r.Delete("/users/{id}", handlers.Auth.DeleteUser)
			r.Post("/users/{id}/disable", handlers.Auth.DisableUser)
		})
	})
}

func healthCheck(store kexswiftdb.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result := map[string]any{
			"status": "ok",
		}

		if err := store.Ping(r.Context()); err != nil {
			result["status"] = "unhealthy"
			result["store"] = map[string]any{"status": "unhealthy", "error": err.Error()}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(result)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
	}
}

func storeDebugHandler(store kexswiftdb.Store, driver string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats := store.Stats(r.Context())
		middleware.WriteJSON(w, http.StatusOK, map[string]any{
			"backend": driver,
			"stats":   stats,
		})
	}
}
