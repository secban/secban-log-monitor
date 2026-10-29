package main

import (
	"flag"
	"os"
	"path"

	"github.com/rs/zerolog/log"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	"gitlab.com/secban/secban-log-monitor/config"
)

const filename = "log-monitor.yaml"

func main() {
	var configDir string

	flag.StringVar(&configDir, "config", "/tmp/secban", "path to config directory")
	flag.Parse()

	defaultConfig := config.DefaultParsedConfig()

	os.MkdirAll(configDir, 0755)
	instance := koanf.New(".")
	parser := yaml.Parser()
	loadDefaultErr := instance.Load(structs.Provider(defaultConfig, "koanf"), nil)
	if loadDefaultErr != nil {
		log.Fatal().Err(loadDefaultErr).Msg("Error loading log monitor config")
	}
	destination := path.Join(configDir, filename)
	marshalled, marshallErr := instance.Marshal(parser)
	if marshallErr != nil {
		log.Fatal().Err(marshallErr).Msg("Error marshalling log monitor config")
	}
	writeErr := os.WriteFile(destination, marshalled, 0644)
	if writeErr != nil {
		log.Fatal().Err(writeErr).Msg("Error writing log monitor config")
	}
}
