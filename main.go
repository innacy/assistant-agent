package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/innacy/assistant-agent/pkg/api"
	auth_pkg "github.com/innacy/assistant-agent/pkg/auth"
	"github.com/innacy/assistant-agent/pkg/config"
	daemonpkg "github.com/innacy/assistant-agent/pkg/daemon"
	"github.com/innacy/assistant-agent/pkg/db"
)

func main() {
	serve := flag.Bool("serve", false, "Start API + UI + daemon")
	daemon := flag.Bool("daemon", false, "Start headless daemon only")
	syncOnce := flag.Bool("sync-once", false, "Run single sync then exit")
	auth := flag.Bool("auth", false, "Run OAuth flow and exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	level, _ := zerolog.ParseLevel(cfg.LogLevel)
	zerolog.SetGlobalLevel(level)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	switch {
	case *auth:
		if err := auth_pkg.RunAuthFlow(cfg.Google); err != nil {
			log.Fatal().Err(err).Msg("auth flow failed")
		}
		log.Info().Msg("Authentication successful! Token saved.")
	case *syncOnce:
		database, err := db.Connect(cfg.DB)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to connect to MongoDB")
		}
		defer database.Close()

		d, err := daemonpkg.New(database, cfg)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create daemon")
		}
		if err := d.RunOnce(context.Background()); err != nil {
			log.Fatal().Err(err).Msg("sync failed")
		}
	case *daemon:
		database, err := db.Connect(cfg.DB)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to connect to MongoDB")
		}
		defer database.Close()

		d, err := daemonpkg.New(database, cfg)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create daemon")
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		if err := d.Run(ctx); err != nil && err != context.Canceled {
			log.Fatal().Err(err).Msg("daemon failed")
		}
	case *serve:
		database, err := db.Connect(cfg.DB)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to connect to MongoDB")
		}
		defer database.Close()

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		var d *daemonpkg.Daemon
		d, err = daemonpkg.New(database, cfg)
		if err != nil {
			log.Warn().Err(err).Msg("daemon unavailable (run --auth first); starting API only")
		} else {
			go func() {
				if err := d.Run(ctx); err != nil && err != context.Canceled {
					log.Error().Err(err).Msg("daemon stopped with error")
				}
			}()
		}

		srv := api.NewServer(database, cfg)
		log.Info().Int("port", cfg.Server.Port).Msg("starting server")
		if err := srv.RunWithContext(ctx); err != nil {
			log.Fatal().Err(err).Msg("server failed")
		}

		if d != nil {
			d.Stop()
		}
		log.Info().Msg("shutdown complete")
	default:
		flag.Usage()
	}
}
