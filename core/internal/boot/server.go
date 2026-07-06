package boot

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/roidmc/quotagate/internal/config"
	tlsutil "github.com/roidmc/quotagate/internal/util/tls"
	"github.com/roidmc/quotagate/pkg/kexswiftdb"
)

func InitHTTPServer(cfg *config.Config, handler http.Handler) *http.Server {
	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	return &http.Server{
		Addr:           addr,
		Handler:        handler,
		ReadTimeout:    time.Duration(cfg.HTTP.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(cfg.HTTP.WriteTimeout) * time.Second,
		MaxHeaderBytes: cfg.HTTP.MaxHeaderBytes,
	}
}

func RunAndWait(srv *http.Server, cfg *config.Config, store kexswiftdb.Store) error {
	go func() {
		var err error
		if cfg.HTTP.TLS.Enabled {
			certFile := cfg.HTTP.TLS.CertFile
			keyFile := cfg.HTTP.TLS.KeyFile

			if certFile == "" || keyFile == "" || !tlsutil.FileExists(certFile) || !tlsutil.FileExists(keyFile) {
				slog.Info("quotagate/boot: TLS enabled but no valid certificate found, generating self-signed certificate...")
				cert, err := tlsutil.GenerateSelfSignedCert("./certs", cfg.Server.Name)
				if err != nil {
					slog.Error("quotagate/boot: failed to generate self-signed certificate", "error", err)
					return
				}
				certFile = cert.CertFile
				keyFile = cert.KeyFile
				slog.Info("quotagate/boot: Self-signed certificate generated", "cert_file", certFile)
			}

			slog.Info("quotagate/boot: Server starting", "scheme", "https", "addr", srv.Addr)
			err = srv.ListenAndServeTLS(certFile, keyFile)
		} else {
			slog.Info("quotagate/boot: Server starting", "scheme", "http", "addr", srv.Addr)
			err = srv.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			slog.Error("quotagate/boot: failed to start server", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("quotagate/boot: Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.HTTP.ShutdownTimeout)*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("quotagate/boot: Server forced to shutdown", "error", err)
	}

	if err := store.Close(); err != nil {
		slog.Warn("quotagate/boot: store close error", "error", err)
	}

	slog.Info("quotagate/boot: Server exited")
	return nil
}
