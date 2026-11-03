package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"blotato-comment-automation/shared/kafka"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Database struct {
		URL string `yaml:"url"`
	} `yaml:"database"`
	Kafka kafka.Config `yaml:"messaging"`
}

func Load(path string) (Config, error) {
	var cfg Config
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
	return cfg, nil
}
