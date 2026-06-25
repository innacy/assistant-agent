package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/innacy/assistant-agent/pkg/api"
	"github.com/innacy/assistant-agent/pkg/config"
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
		fmt.Println("TODO: OAuth flow")
	case *syncOnce:
		fmt.Println("TODO: single sync")
	case *daemon:
		fmt.Println("TODO: daemon mode")
	case *serve:
		database, err := db.Connect(cfg.DB)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to connect to MongoDB")
		}
		defer database.Close()

		srv := api.NewServer(database, cfg)
		log.Info().Int("port", cfg.Server.Port).Msg("starting server")
		if err := srv.Run(); err != nil {
			log.Fatal().Err(err).Msg("server failed")
		}
	default:
		flag.Usage()
	}
}
