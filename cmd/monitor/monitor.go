package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/secban/secban-agent/version"
	"gitlab.com/secban/secban-log-monitor/config"
	"gitlab.com/secban/secban-log-monitor/monitor"
)

func main() {
	version.CLI()

	var configDir string

	flag.StringVar(&configDir, "config", "/etc/secban", "path to config directory")
	flag.Parse()

	conf, confErr := config.ParseConfig(configDir)
	if confErr != nil {
		panic(confErr)
	}

	level, parseLevelErr := zerolog.ParseLevel(conf.LogLevel)
	if parseLevelErr != nil {
		level = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(level)

	mon, monErr := monitor.New(conf)
	if monErr != nil {
		log.Fatal().Err(monErr).Msg("failed to initialize monitor")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info().
		Str("agent_url", conf.AgentURL).
		Str("log_level", level.String()).
		Int("watchers", len(conf.Watchers)).
		Msg("monitor service starting")

	mon.Start(ctx)

	log.Info().Msg("monitor service stopped")
}
