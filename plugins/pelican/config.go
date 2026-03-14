package pelican

import (
	"fmt"

	sharedcfg "github.com/minekube/gate-plugin-template/plugins/sharedconfig"
)

type Config struct {
	Token    string            `yaml:"token" json:"token"`
	URL      string            `yaml:"url" json:"url"`
	Prefix   string            `yaml:"prefix" json:"prefix"`
	Autostop bool              `yaml:"autostop" json:"autostop"`
	Delay    int               `yaml:"delay" json:"delay"`
	Servers  map[string]string `yaml:"servers,omitempty" json:"servers,omitempty"`
	Messages MessagesConfig    `yaml:"messages" json:"messages"`
}

type MessagesConfig struct {
	ServerStartingWait string `yaml:"serverStartingWait" json:"serverStartingWait"`
	ErrorStarting      string `yaml:"errorStarting" json:"errorStarting"`
	StartingServer     string `yaml:"startingServer" json:"startingServer"`
}

var DefaultConfig = Config{
	Token:    "Your Pelican token",
	URL:      "https://demo.pelican.dev",
	Prefix:   "<gray>[<aqua>Pelican</aqua>]</gray> ",
	Autostop: true,
	Delay:    60,
	Servers: map[string]string{
		"server1": "The UUID of the server you want to connect to",
	},
	Messages: MessagesConfig{
		ServerStartingWait: "<yellow>Server is starting, please wait...</yellow>",
		ErrorStarting:      "<red>Error starting server</red>",
		StartingServer:     "<yellow>Starting server...</yellow>",
	},
}

func LoadConfig() (*Config, error) {
	cfg := func() Config { return DefaultConfig }()
	rootCfg, err := sharedcfg.Load()
	if err != nil {
		return &cfg, fmt.Errorf("load shared config: %w", err)
	}

	p := rootCfg.Plugins.Pelican
	cfg.Token = p.Token
	cfg.URL = p.URL
	cfg.Prefix = p.Prefix
	cfg.Autostop = p.AutoStop
	cfg.Delay = p.Delay
	cfg.Servers = p.Servers
	cfg.Messages = MessagesConfig{
		ServerStartingWait: p.Messages.ServerStartingWait,
		ErrorStarting:      p.Messages.ErrorStarting,
		StartingServer:     p.Messages.StartingServer,
	}

	if cfg.Servers == nil {
		cfg.Servers = map[string]string{}
	}

	return &cfg, nil
}
