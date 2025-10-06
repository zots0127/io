package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/zots0127/io/pkg/api"
	"github.com/zots0127/io/pkg/config"
	"github.com/zots0127/io/pkg/service"
	storageservice "github.com/zots0127/io/pkg/storage/service"
	"github.com/zots0127/io/pkg/web"
)

func main() {
	// Parse command line flags
	var (
		configFile = flag.String("config", "", "Configuration file path")
		host       = flag.String("host", "0.0.0.0", "Web server host")
		port       = flag.Int("port", 8080, "Web server port")
		debug      = flag.Bool("debug", false, "Enable debug mode")
		https      = flag.Bool("https", false, "Enable HTTPS")
		certFile   = flag.String("cert", "", "SSL certificate file (required for HTTPS)")
		keyFile    = flag.String("key", "", "SSL private key file (required for HTTPS)")
		help       = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	fmt.Println("🚀 IO Storage System - Web Interface")
	fmt.Println("=====================================")

	// Load configuration
	appConfig, err := loadConfiguration(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create services
	fileService, err := createFileService(appConfig)
	if err != nil {
		log.Fatalf("Failed to create file service: %v", err)
	}

	searchService, err := createSearchService(appConfig)
	if err != nil {
		log.Fatalf("Failed to create search service: %v", err)
	}

	// Create batch service
	serviceConfig := service.DefaultServiceConfig()
	batchService := service.NewBatchService(fileService, searchService, serviceConfig)

	// Create batch API
	batchAPI := api.NewBatchAPI(batchService)

	// Configure web server
	webConfig := &web.Config{
		Host:         *host,
		Port:         *port,
		TemplatePath: "./pkg/web/templates",
		StaticPath:   "./pkg/web/static",
		Debug:        *debug,
		EnableAuth:   false,
		EnableHTTPS:  *https,
		SSLCert:      *certFile,
		SSLKey:       *keyFile,
	}

	// Override with config file values if available
	if appConfig != nil {
		config := appConfig.GetConfig()
		if config.Server.Host != "" {
			webConfig.Host = config.Server.Host
		}
		if config.Server.Port != "" {
			// Parse string port to int for web config
			if port, err := strconv.Atoi(config.Server.Port); err == nil {
				webConfig.Port = port
			}
		}
		// Use storage path from config if available
		if config.Storage.Path != "" {
			// We could use this for file service initialization here
		}
	}

	// Create web integration
	webIntegration, err := web.NewWebIntegration(batchAPI, fileService, webConfig)
	if err != nil {
		log.Fatalf("Failed to create web integration: %v", err)
	}

	// Start services
	fmt.Printf("📦 Services Initialized\n")
	fmt.Printf("   File Service: ✅\n")
	fmt.Printf("   Search Service: ✅\n")
	fmt.Printf("   Batch Service: ✅\n")
	fmt.Printf("   Web Interface: ✅\n\n")

	// Print startup information
	fmt.Printf("🌐 Starting Web Interface\n")
	fmt.Printf("   Mode: %s\n", getModeString(*debug))
	fmt.Printf("   Address: %s\n", getAddressString(*host, *port, *https))
	fmt.Printf("   Templates: %s\n", webConfig.TemplatePath)
	fmt.Printf("   Static: %s\n", webConfig.StaticPath)
	fmt.Printf("   Authentication: %s\n", getAuthString(webConfig.EnableAuth))

	if *https {
		fmt.Printf("   SSL Certificate: %s\n", *certFile)
		fmt.Printf("   SSL Key: %s\n", *keyFile)
	}

	fmt.Println()

	// Start web interface in a goroutine
	go func() {
		if err := webIntegration.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start web interface: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n🛑 Shutdown signal received, shutting down gracefully...")

	// Create context for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown web interface
	if err := webIntegration.Stop(ctx); err != nil {
		log.Printf("Error during web interface shutdown: %v", err)
	}

	fmt.Println("✅ Web interface stopped successfully")
	fmt.Println("👋 Goodbye!")
}

func loadConfiguration(configFile string) (*config.ConfigManager, error) {
	if configFile == "" {
		// Try to find default config files
		defaultFiles := []string{
			"config.yaml",
			"config.yml",
			"config.json",
			"pkg/config/config.yaml",
			"pkg/config/config.yml",
			"pkg/config/config.json",
		}

		for _, file := range defaultFiles {
			if _, err := os.Stat(file); err == nil {
				configFile = file
				break
			}
		}

		if configFile == "" {
			fmt.Println("⚠️  No configuration file found, using defaults")
			return nil, nil
		}
	}

	fmt.Printf("📄 Loading configuration from: %s\n", configFile)
	configManager := config.NewConfigManager()
	_, err := configManager.Load(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return configManager, nil
}

func createFileService(appConfig *config.ConfigManager) (service.FileService, error) {
	// Create storage implementation
	storage := storageservice.NewStorage("./data") // Default storage path

	// Create service configuration
	serviceConfig := service.DefaultServiceConfig()

	// For now, create a basic file service without metadata repository
	// In a real implementation, this would be configured based on the config
	return service.NewFileService(storage, nil, serviceConfig), nil
}

func createSearchService(appConfig *config.ConfigManager) (service.SearchService, error) {
	// Create service configuration
	serviceConfig := service.DefaultServiceConfig()

	// For now, create a basic search service without metadata repository
	// In a real implementation, this would be configured based on the config
	return service.NewSearchService(nil, serviceConfig), nil
}

func showHelp() {
	fmt.Println("IO Storage System - Web Interface")
	fmt.Println("================================")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("    go run cmd/web/main.go [OPTIONS]")
	fmt.Println()
	fmt.Println("OPTIONS:")
	fmt.Println("    -config string    Configuration file path")
	fmt.Println("    -host string      Web server host (default \"0.0.0.0\")")
	fmt.Println("    -port int         Web server port (default 8080)")
	fmt.Println("    -debug            Enable debug mode")
	fmt.Println("    -https            Enable HTTPS")
	fmt.Println("    -cert string      SSL certificate file (required for HTTPS)")
	fmt.Println("    -key string       SSL private key file (required for HTTPS)")
	fmt.Println("    -help             Show this help message")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("    # Start with default settings")
	fmt.Println("    go run cmd/web/main.go")
	fmt.Println()
	fmt.Println("    # Start with custom port and debug mode")
	fmt.Println("    go run cmd/web/main.go -port 9090 -debug")
	fmt.Println()
	fmt.Println("    # Start with HTTPS")
	fmt.Println("    go run cmd/web/main.go -https -cert server.crt -key server.key")
	fmt.Println()
	fmt.Println("    # Start with configuration file")
	fmt.Println("    go run cmd/web/main.go -config config.yaml")
	fmt.Println()
	fmt.Println("ENVIRONMENT VARIABLES:")
	fmt.Println("    WEB_HOST         Web server host")
	fmt.Println("    WEB_PORT         Web server port")
	fmt.Println("    WEB_DEBUG        Enable debug mode (true/false)")
	fmt.Println("    WEB_HTTPS        Enable HTTPS (true/false)")
	fmt.Println("    WEB_CERT         SSL certificate file")
	fmt.Println("    WEB_KEY          SSL private key file")
}

func getModeString(debug bool) string {
	if debug {
		return "Development"
	}
	return "Production"
}

func getAddressString(host string, port int, https bool) string {
	protocol := "http"
	if https {
		protocol = "https"
	}

	if host == "0.0.0.0" {
		host = "localhost"
	}

	return fmt.Sprintf("%s://%s:%d", protocol, host, port)
}

func getAuthString(enabled bool) string {
	if enabled {
		return "Enabled"
	}
	return "Disabled"
}