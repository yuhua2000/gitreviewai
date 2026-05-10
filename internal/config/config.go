package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// GitLab 配置
	GitLabURL   string `yaml:"gitlab_url"`
	GitLabToken string `yaml:"gitlab_token"`

	// OpenAI 配置
	OpenAIAPIKey  string `yaml:"openai_api_key"`
	OpenAIModel   string `yaml:"openai_model"`
	OpenAIBaseURL string `yaml:"openai_base_url"`

	// 服务配置
	Port         int    `yaml:"port"`
	WebhookToken string `yaml:"webhook_token"`

	// 审核配置
	MaxLineComments int      `yaml:"max_line_comments"`
	IgnorePaths     []string `yaml:"ignore_paths"`

	LogLevel string `yaml:"log_level"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{
		// 默认值
		GitLabURL:       "https://gitlab.com",
		OpenAIModel:     "gpt-4o",
		Port:            8080,
		MaxLineComments: 50,
		IgnorePaths:     []string{".git", "vendor", "node_modules"},
		LogLevel:        "info",
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	return cfg, nil
}
