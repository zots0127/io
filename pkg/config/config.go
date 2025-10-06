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
		Mode string `yaml:"mode"` // native, s3, or hybrid
	} `yaml:"api"`
	S3 struct {
		Enabled   bool   `yaml:"enabled"`
		Port      string `yaml:"port"`
		AccessKey string `yaml:"access_key"`
		SecretKey string `yaml:"secret_key"`
		Region    string `yaml:"region"`
	} `yaml:"s3"`
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
	
	// Override from environment variables if set
	if envAPIKey := os.Getenv("IO_API_KEY"); envAPIKey != "" {
		config.API.Key = envAPIKey
	}
	
	if envS3AccessKey := os.Getenv("IO_S3_ACCESS_KEY"); envS3AccessKey != "" {
		config.S3.AccessKey = envS3AccessKey
	}
	
	if envS3SecretKey := os.Getenv("IO_S3_SECRET_KEY"); envS3SecretKey != "" {
		config.S3.SecretKey = envS3SecretKey
	}
	
	// Set defaults for S3 if not specified
	if config.S3.Port == "" {
		config.S3.Port = "9000"
	}
	if config.S3.Region == "" {
		config.S3.Region = "us-east-1"
	}
	if config.API.Mode == "" {
		config.API.Mode = "native"
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
			Mode string `yaml:"mode"`
		}{
			Port: "8080",
			Key:  apiKey,
			Mode: "native",
		},
		S3: struct {
			Enabled   bool   `yaml:"enabled"`
			Port      string `yaml:"port"`
			AccessKey string `yaml:"access_key"`
			SecretKey string `yaml:"secret_key"`
			Region    string `yaml:"region"`
		}{
			Enabled:   false,
			Port:      "9000",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Region:    "us-east-1",
		},
	}
}