package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"blotato-comment-automation/internal/feature/conversation"
	"blotato-comment-automation/internal/feature/delivery"
	"blotato-comment-automation/internal/feature/execution"
	"blotato-comment-automation/internal/feature/matching"
	"blotato-comment-automation/shared/kafka"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Execution     execution.Config    `yaml:"execution"`
	Conversations conversation.Config `yaml:"conversations"`
	Delivery      delivery.Config     `yaml:"delivery"`
	Database      struct {
		URL string `yaml:"url"`
	} `yaml:"database"`
	Kafka    kafka.Config    `yaml:"messaging"`
	Matching matching.Config `yaml:"matching"`
	HTTP     struct {
		Address string `yaml:"address"`
		Token   string `yaml:"token"`
	} `yaml:"http"`
}

func Load(path string) (Config, error) {
	cfg := Config{Matching: matching.DefaultConfig(), Execution: execution.DefaultConfig(), Conversations: conversation.DefaultConfig()}
	f, err := os.Open(path)
	if err != nil {
		return cfg, err
	}
	defer f.Close()
	decoder := yaml.NewDecoder(f)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("decode %s: %w", path, err)
	}
	if strings.TrimSpace(cfg.Database.URL) == "" {
		return cfg, errors.New("database.url is required")
	}
	if len(cfg.Kafka.Brokers) == 0 {
		return cfg, errors.New("messaging.broker is required")
	}
	for i, broker := range cfg.Kafka.Brokers {
		cfg.Kafka.Brokers[i] = strings.TrimSpace(broker)
		if cfg.Kafka.Brokers[i] == "" {
			return cfg, errors.New("messaging.broker contains an empty address")
		}
	}
	if strings.TrimSpace(cfg.Kafka.GroupID) == "" || strings.TrimSpace(cfg.Kafka.Topic) == "" {
		return cfg, errors.New("messaging.group_id and messaging.topic are required")
	}
	if strings.TrimSpace(cfg.Kafka.RulesTopic) == "" || cfg.Kafka.RulesTopic == cfg.Kafka.Topic {
		return cfg, errors.New("messaging.rules_topic must be separate from the inbound topic")
	}
	if cfg.HTTP.Address == "" || len(cfg.HTTP.Token) < 16 {
		return cfg, errors.New("http.address and an internal http.token of at least 16 characters are required")
	}
	if err := cfg.Matching.Validate(); err != nil {
		return cfg, err
	}
	if cfg.Kafka.Partitions < 1 {
		return cfg, errors.New("messaging.partitions is required")
	}
	if err := cfg.Execution.Validate(); err != nil {
		return cfg, err
	}
	if err := cfg.Conversations.Validate(); err != nil {
		return cfg, err
	}
	if err := cfg.Delivery.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}
