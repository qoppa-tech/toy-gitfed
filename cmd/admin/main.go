package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/qoppa-tech/gitfed/internal/admin/seed"
	"github.com/qoppa-tech/gitfed/internal/config"
	"github.com/qoppa-tech/gitfed/internal/database"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid config", slog.Any("error", err))
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg.Database)
	if err != nil {
		slog.Error("database connect failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer db.Close()

	switch os.Args[1] {
	case "seed":
		if err := cfg.ValidateSeed(); err != nil {
			slog.Error("seed config validation failed", slog.Any("error", err))
			os.Exit(1)
		}

		if err := seed.Run(ctx, db, seed.Input{
			AdminName:     cfg.SeedAdminName,
			AdminUsername: cfg.SeedAdminUsername,
			AdminEmail:    cfg.SeedAdminEmail,
			AdminPassword: cfg.SeedAdminPassword,
		}); err != nil {
			slog.Error("seed failed", slog.Any("error", err))
			os.Exit(1)
		}
		slog.Info("seed completed")
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println("usage: go run ./cmd/admin <seed>")
}
