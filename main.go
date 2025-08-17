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
	
	if err := os.MkdirAll(config.Storage.Path, 0755); err != nil {
		log.Fatal("Failed to create storage directory:", err)
	}
	
	storage := NewStorage(config.Storage.Path)
	api := NewAPI(storage, config.API.Key)
	
	router := gin.Default()
	api.RegisterRoutes(router)
	
	log.Printf("Starting server on port %s", config.API.Port)
	if err := router.Run(":" + config.API.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}