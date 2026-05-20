package config

import (
	"github.com/spf13/viper"
)

type ModelConfig struct {
	Type        string  `mapstructure:"type"`
	APIKey      string  `mapstructure:"api_key"`
	BaseURL     string  `mapstructure:"base_url"`
	ModelName   string  `mapstructure:"model_name"`
	Temperature float32 `mapstructure:"temperature"`
}

type MCPToolConfig struct {
	Name    string `mapstructure:"name"`
	Enabled bool   `mapstructure:"enabled"`
	APIKey  string `mapstructure:"api_key"`
}

type Config struct {
	DefaultModel string                 `mapstructure:"default_model"`
	Models       map[string]ModelConfig `mapstructure:"models"`
	MCPTools     []MCPToolConfig        `mapstructure:"mcp_tools"`
	MemoryPath   string                 `mapstructure:"memory_path"`
}

func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
