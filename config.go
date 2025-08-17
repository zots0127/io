package main

import (
	"crypto/sha256"
	"encoding/hex"
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
	
	// Override API key from environment variable if set
	if envAPIKey := os.Getenv("IO_API_KEY"); envAPIKey != "" {
		config.API.Key = envAPIKey
	}
	
	// Hash the API key for internal use (still use plain key for comparison)
	// This is just for logging purposes to avoid exposing the key
	if config.API.Key != "" {
		hasher := sha256.New()
		hasher.Write([]byte(config.API.Key))
		hashBytes := hasher.Sum(nil)[:8] // Use first 8 bytes for display
		log.Printf("API Key configured (hash prefix: %s...)", hex.EncodeToString(hashBytes))
	}
	
	return &config
}

func defaultConfig() *Config {
	// Try to get API key from environment variable first
	apiKey := os.Getenv("IO_API_KEY")
	if apiKey == "" {
		log.Fatal("API key must be set via IO_API_KEY environment variable or config file")
	}
	
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
			Key:  apiKey,
		},
	}
}