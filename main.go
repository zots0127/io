package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	config := LoadConfig()

	if err := InitDB(config.Storage.Database); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer CloseDB()

	// Initialize S3 tables if S3 mode is enabled
	if config.API.Mode == "s3" || config.API.Mode == "hybrid" {
		if err := InitS3Tables(); err != nil {
			log.Fatal("Failed to initialize S3 tables:", err)
		}
		if err := InitMultipartUploadDB(); err != nil {
			log.Fatal("Failed to initialize multipart tables:", err)
		}
	}

	if err := os.MkdirAll(config.Storage.Path, 0755); err != nil {
		log.Fatal("Failed to create storage directory:", err)
	}

	storage := NewStorage(config.Storage.Path)

	// Initialize metadata database
	var metadataDB *MetadataDB
	if config.Storage.Database != "" {
		metadataDBPath := config.Storage.Database + ".metadata"
		var err error
		metadataDB, err = NewMetadataDB(metadataDBPath)
		if err != nil {
			log.Printf("Warning: Failed to initialize metadata database: %v", err)
			metadataDB = nil
		} else {
			defer metadataDB.Close()
		}
	}

	// Create API instance
	api := NewAPI(storage, metadataDB, config.API.Key)

	// Create Web interface instance
	webInterface := NewWebInterface(api, storage, metadataDB)
	
	switch config.API.Mode {
	case "native":
		// Native API with Web interface
		router := gin.Default()

		// Register API routes
		api.RegisterRoutes(router)

		// Register Web interface routes
		webInterface.RegisterRoutes(router)

		log.Printf("Starting IO Storage with Web interface on port %s", config.API.Port)
		log.Printf("Web interface: http://localhost:%s", config.API.Port)
		log.Printf("API endpoints: http://localhost:%s/api", config.API.Port)

		if err := router.Run(":" + config.API.Port); err != nil {
			log.Fatal("Failed to start server:", err)
		}
		
	case "s3":
		// S3 API only
		s3Config := &S3Config{
			AccessKey: config.S3.AccessKey,
			SecretKey: config.S3.SecretKey,
			Region:    config.S3.Region,
			Port:      config.S3.Port,
		}
		s3api := NewS3API(storage, s3Config)
		router := gin.Default()
		s3api.RegisterRoutes(router)
		
		log.Printf("Starting S3-compatible API server on port %s", config.S3.Port)
		log.Printf("Access Key: %s", config.S3.AccessKey)
		log.Printf("Region: %s", config.S3.Region)
		if err := router.Run(":" + config.S3.Port); err != nil {
			log.Fatal("Failed to start S3 server:", err)
		}
		
	case "hybrid":
		// Both APIs running on different ports
		errChan := make(chan error, 2)
		
		// Start native API with Web interface
		go func() {
			api := NewAPI(storage, metadataDB, config.API.Key)
			webInterface := NewWebInterface(api, storage, metadataDB)
			router := gin.New()
			router.Use(gin.Recovery())
			api.RegisterRoutes(router)
			webInterface.RegisterRoutes(router)

			log.Printf("Starting native API server with Web interface on port %s", config.API.Port)
			log.Printf("Web interface: http://localhost:%s", config.API.Port)
			if err := router.Run(":" + config.API.Port); err != nil {
				errChan <- err
			}
		}()
		
		// Start S3 API
		go func() {
			s3Config := &S3Config{
				AccessKey: config.S3.AccessKey,
				SecretKey: config.S3.SecretKey,
				Region:    config.S3.Region,
				Port:      config.S3.Port,
			}
			s3api := NewS3API(storage, s3Config)
			router := gin.New()
			router.Use(gin.Recovery())
			s3api.RegisterRoutes(router)
			
			log.Printf("Starting S3-compatible API server on port %s", config.S3.Port)
			log.Printf("Access Key: %s", config.S3.AccessKey)
			log.Printf("Region: %s", config.S3.Region)
			if err := router.Run(":" + config.S3.Port); err != nil {
				errChan <- err
			}
		}()
		
		// Wait for any error
		err := <-errChan
		log.Fatal("Server error:", err)
		
	default:
		log.Fatalf("Unknown API mode: %s", config.API.Mode)
	}
}