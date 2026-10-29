package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/secban/secban-agent/profiler"
	"gitlab.com/secban/secban-agent/version"
	"gitlab.com/secban/secban-log-monitor/config"
	"gitlab.com/secban/secban-log-monitor/monitor"
)

func main() {
	version.CLI()

	conf := loadConf()

	zerolog.SetGlobalLevel(conf.LogLevel)

	mon, monErr := monitor.New(conf)
	if monErr != nil {
		log.Fatal().Err(monErr).Msg("failed to initialize monitor")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if conf.Profiler != "" {
		go profiler.Listen(ctx, conf.Profiler)
	}

	mon.Start(ctx)

	log.Info().Msg("monitor service stopped")
}

func loadConf() config.Config {
	var configDir string
	//nolint:revive,nolintlint // Not a deep-exit, just split from main() for readability.
	flag.StringVar(&configDir, "config", "/etc/secban", "path to config directory")
	//nolint:revive,nolintlint
	flag.Parse()

	conf, confErr := config.LoadConfig(configDir)
	if confErr != nil {
		log.Fatal().Err(confErr).Msg("failed to load config")
	}

	return conf
}
