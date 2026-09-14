package config

import "testing"

func TestLoadExampleYAML(t *testing.T) {
	var cfg Config
	if err := cfg.loadFromSource("example.yaml"); err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Name != "irc.example.com" || cfg.ListenIRC.Port != 6667 {
		t.Fatalf("Unexpected server configuration: %+v", cfg)
	}
	if !cfg.ListenIRC.Enabled || cfg.ListenTLS.Enabled {
		t.Fatal("YAML listener flags were not preserved")
	}
	if len(cfg.Bots.BearerTokens) != 2 || len(cfg.Operators) != 2 {
		t.Fatal("YAML lists were not preserved")
	}
	if len(cfg.Plugins) == 0 || cfg.Plugins[0].Config["log_level"] != "info" {
		t.Fatal("YAML nested plugin configuration was not preserved")
	}
}
