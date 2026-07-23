package boot

import (
	"fmt"
	"log/slog"

	"github.com/roidmc/quotagate/internal/config"
	"github.com/roidmc/quotagate/internal/router"
	"github.com/roidmc/quotagate/internal/service"
)

// Run initializes all components and starts the HTTP server.
// It is the main entry point for the quotagate server.
func Run() error {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := InitDB(cfg)
	if err != nil {
		return fmt.Errorf("init database: %w", err)
	}
	defer StopEmbeddedDB() // no-op unless database.embedded=true

	store, err := InitStore(cfg)
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}

	eventBus, err := InitEventBus(cfg)
	if err != nil {
		return fmt.Errorf("init event bus: %w", err)
	}
	defer func() {
		if eventBus != nil {
			eventBus.Close()
		}
	}()

	repos, err := InitRepos(db)
	if err != nil {
		return fmt.Errorf("init repos: %w", err)
	}

	jwtManager, err := InitJWT(cfg)
	if err != nil {
		return fmt.Errorf("init jwt: %w", err)
	}

	tokenRevoker := service.NewStoreTokenRevoker(store)
	tokenIssuer := service.NewTokenIssuer(jwtManager, tokenRevoker, int64(cfg.JWT.AccessExpiry))

	identityReg, err := InitIdentity(cfg, repos, jwtManager, store)
	if err != nil {
		return fmt.Errorf("init identity: %w", err)
	}

	authzManager, err := InitAuthz(db, eventBus, repos.RoleRepo, repos.RouteMetaRepo)
	if err != nil {
		return fmt.Errorf("init authz: %w", err)
	}
	defer func() {
		if err := authzManager.Close(); err != nil {
			slog.Warn("quotagate/boot: authz manager close error", "error", err)
		}
	}()

	handlers, err := InitHandlers(repos, identityReg, tokenIssuer, tokenRevoker, authzManager, cfg, store)
	if err != nil {
		return fmt.Errorf("init handlers: %w", err)
	}

	engine, err := InitEngine(cfg)
	if err != nil {
		return fmt.Errorf("init engine: %w", err)
	}

	router.Setup(engine, handlers, tokenIssuer, authzManager, store, cfg.Store.Driver)
	srv := InitHTTPServer(cfg, engine)

	slog.Info("quotagate/boot: server initialized")
	return RunAndWait(srv, cfg, store)
}
