package main

import (
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Storage struct {
		Path     string `yaml:"path"`
		Database string `yaml:"database"`
	} `yaml:"storage"`
	API struct {
		Port string `yaml:"port"`
		Key  string `yaml:"key"`
	} `yaml:"api"`
}

func LoadConfig() *Config {
	configPath := "config.yaml"
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		configPath = envPath
	}
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("Failed to read config file, using defaults: %v", err)
		return defaultConfig()
	}
	
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Printf("Failed to parse config file, using defaults: %v", err)
		return defaultConfig()
	}
	
	return &config
}

func defaultConfig() *Config {
	return &Config{
		Storage: struct {
			Path     string `yaml:"path"`
			Database string `yaml:"database"`
		}{
			Path:     "./storage",
			Database: "./storage.db",
		},
		API: struct {
			Port string `yaml:"port"`
			Key  string `yaml:"key"`
		}{
			Port: "8080",
			Key:  "default-api-key",
		},
	}
}