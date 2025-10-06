package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zots0127/io/pkg/ai"
	"github.com/zots0127/io/pkg/api/handler"
	"github.com/zots0127/io/pkg/middleware"
	"github.com/zots0127/io/pkg/metadata/repository"
	"github.com/zots0127/io/pkg/storage/service"
)

func TestAIIntegration(t *testing.T) {
	// 创建临时目录和数据库
	tempDir, err := os.MkdirTemp("", "ai_test")
	if err != nil {
		t.Fatal("Failed to create temp directory:", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	storagePath := filepath.Join(tempDir, "storage")

	// 初始化存储和元数据
	storage := service.NewStorage(storagePath)
	metadataRepo, err := repository.NewMetadataRepository(dbPath)
	if err != nil {
		t.Fatal("Failed to initialize metadata repository:", err)
	}
	defer metadataRepo.Close()

	// 初始化AI服务
	aiService := ai.NewAIService(metadataRepo, nil)
	aiAPI := ai.NewAPI(aiService, nil)

	// 初始化API
	api := handler.NewAPI(storage, metadataRepo)

	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// 注册路由
	aiAPI.RegisterRoutes(router, &middleware.Config{})
	api.RegisterRoutes(router)

	// 测试根路径
	t.Run("Root Endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Root endpoint failed with status %d: %s", w.Code, w.Body.String())
		}

		var rootResp map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &rootResp)
		if err != nil {
			t.Fatal("Failed to parse root response:", err)
		}

		version := rootResp["version"].(string)
		if version != "v1.3.0-alpha (AI Enhanced)" {
			t.Errorf("Expected version 'v1.3.0-alpha (AI Enhanced)', got: %s", version)
		}

		features := rootResp["features"].(map[string]interface{})
		if features["ai_classify"] == nil {
			t.Error("AI classify feature should be listed")
		}

		if features["ai_analyze"] == nil {
			t.Error("AI analyze feature should be listed")
		}

		if features["ai_search"] == nil {
			t.Error("AI search feature should be listed")
		}

		t.Logf("Root endpoint result: Version %s, Features: %+v", version, features)
	})

	// 测试AI健康检查
	t.Run("AI Health Check", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/ai/health", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Health check failed with status %d: %s", w.Code, w.Body.String())
		}

		var healthResp map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &healthResp)
		if err != nil {
			t.Fatal("Failed to parse health response:", err)
		}

		if healthResp["success"] != true {
			t.Error("Health check should be successful")
		}

		data := healthResp["data"].(map[string]interface{})
		if data["status"] != "healthy" {
			t.Errorf("Expected healthy status, got: %v", data["status"])
		}

		t.Logf("Health check result: %+v", healthResp)
	})

	// 测试AI统计
	t.Run("AI Stats", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/ai/stats", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Stats failed with status %d: %s", w.Code, w.Body.String())
		}

		var statsResp map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &statsResp)
		if err != nil {
			t.Fatal("Failed to parse stats response:", err)
		}

		if statsResp["success"] != true {
			t.Error("Stats should be successful")
		}

		t.Logf("Stats result: %+v", statsResp)
	})

	// 测试AI配置
	t.Run("AI Config", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/ai/config", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Config failed with status %d: %s", w.Code, w.Body.String())
		}

		var configResp map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &configResp)
		if err != nil {
			t.Fatal("Failed to parse config response:", err)
		}

		if configResp["success"] != true {
			t.Error("Config should be successful")
		}

		t.Logf("Config result: %+v", configResp)
	})
}