package config

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

// Config all settings
type Config struct {
	Server `toml:"server"`
	Slack  `toml:"slack"`
}

// Server port
type Server struct {
	Port string `toml:"port"`
}

type Slack struct {
	Token       string `toml:"token"`
	Year        int    `toml:"year"`
	PostChannel string `toml:"post_channel"`
}

// New create config
func New(env string) (*Config, error) {
	// まぁ、こんなにディレクトリ固定していいんだっけ話はあるよね。ダメなんだけど。
	configPath := fmt.Sprintf("_tools/%s/config.toml", env)
	config := &Config{}
	_, err := toml.DecodeFile(configPath, config)
	return config, err
}
