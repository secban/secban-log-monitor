package config

func defaultParsedConfig() ParsedConfig {
	return ParsedConfig{
		AgentURL: defaultAgentURL,
		LogLevel: defaultLogLevel,
		Readers:  make(map[string]Reader),
		Watchers: make(map[string]Watcher),
		Matchers: make(map[string]Matcher),
	}
}
