package main

import (
	"log/slog"

	"github.com/roidmc/quotagate/internal/boot"
)

func main() {
	if err := boot.Run(); err != nil {
		slog.Error("quotagate: fatal error", "error", err)
	}
}
