package config

func defaultParsedConfig() ParsedConfig {
	return ParsedConfig{
		AgentURL: "unix:///var/run/secban/agent.sock",
		Profiler: "localhost:6061",
		LogLevel: "info",
		Readers: map[string]Reader{
			"journalctl": journalctlReader(),
			"tail":       tailReader(),
		},
	}
}

func journalctlReader() Reader {
	return Reader{
		Name:    "journalctl",
		Command: "journalctl",
		Args: []string{
			"-afb",
			"-p",
			"info",
			"-n0",
			"-o",
			"cat",
		},
		TestArgs: []string{
			"-b",
			"-p",
			"info",
			"-n500",
			"-o",
			"cat",
		},
		Env: []string{"LANG=C"},
	}
}

func tailReader() Reader {
	return Reader{
		Name:    "tail",
		Command: "tail",
		Args: []string{
			"--follow=name",
			"--retry",
			"--quiet",
			"-n0",
		},
		TestArgs: []string{
			"-n",
			"500",
		},
		Env: []string{"LANG=C"},
	}
}
