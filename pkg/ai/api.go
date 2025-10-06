package ai

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zots0127/io/pkg/middleware"
	"github.com/zots0127/io/pkg/types"
)

// API AI服务API处理器
type API struct {
	aiService AIService
	config    *APIConfig
}

// APIConfig API配置
type APIConfig struct {
	BasePath        string `json:"base_path" yaml:"base_path"`
	EnableBatch     bool   `json:"enable_batch" yaml:"enable_batch"`
	EnableInsights  bool   `json:"enable_insights" yaml:"enable_insights"`
	EnableSearch    bool   `json:"enable_search" yaml:"enable_search"`
	MaxRequestSize  int64  `json:"max_request_size" yaml:"max_request_size"`
	EnableCORS      bool   `json:"enable_cors" yaml:"enable_cors"`
	EnableAuth      bool   `json:"enable_auth" yaml:"enable_auth"`
}

// ClassificationRequest 分类请求
type ClassificationRequest struct {
	SHA1     string `json:"sha1" binding:"required"`
	Filename string `json:"filename" binding:"required"`
	Size     int64  `json:"size" binding:"required"`
	MimeType string `json:"mime_type"`
}

// AnalysisRequest 分析请求
type AnalysisRequest struct {
	SHA1     string `json:"sha1" binding:"required"`
	Filename string `json:"filename" binding:"required"`
	Size     int64  `json:"size" binding:"required"`
	MimeType string `json:"mime_type"`
	Deep     bool   `json:"deep"`
}

// BatchClassificationRequest 批量分类请求
type BatchClassificationRequest struct {
	Files []ClassificationRequest `json:"files" binding:"required,min=1,max=100"`
}

// BatchAnalysisRequest 批量分析请求
type BatchAnalysisRequest struct {
	Files []AnalysisRequest `json:"files" binding:"required,min=1,max=50"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Tags      []string `json:"tags" binding:"required,min=1"`
	Limit     int      `json:"limit" binding:"min=1,max=100"`
	Offset    int      `json:"offset" binding:"min=0"`
	Category  string   `json:"category"`
	SortBy    string   `json:"sortBy" binding:"omitempty,oneof=name size created_at"`
	SortOrder string   `json:"sortOrder" binding:"omitempty,oneof=asc desc"`
}

// SimilarityRequest 相似性请求
type SimilarityRequest struct {
	SHA1  string `json:"sha1" binding:"required"`
	Limit int    `json:"limit" binding:"min=1,max=20"`
}

// NewAPI 创建AI API
func NewAPI(aiService AIService, config *APIConfig) *API {
	if config == nil {
		config = &APIConfig{
			BasePath:       "/api/v1/ai",
			EnableBatch:    true,
			EnableInsights: true,
			EnableSearch:   true,
			MaxRequestSize: 100 * 1024 * 1024, // 100MB
			EnableCORS:     true,
			EnableAuth:     false,
		}
	}

	return &API{
		aiService: aiService,
		config:    config,
	}
}

// RegisterRoutes 注册路由
func (api *API) RegisterRoutes(router *gin.Engine, middlewareConfig *middleware.Config) {
	basePath := api.config.BasePath
	ai := router.Group(basePath)

	// 应用中间件
	if middlewareConfig.EnableAuth && api.config.EnableAuth {
		// ai.Use(api.authMiddleware())
	}
	if middlewareConfig.EnableRateLimit {
		// ai.Use(api.rateLimitMiddleware())
	}
	if api.config.EnableCORS {
		// ai.Use(api.corsMiddleware())
	}

	// 文件分类
	{
		ai.POST("/classify", api.classifyFile)
		ai.GET("/classify/:sha1", api.getClassification)
		ai.POST("/batch/classify", api.batchClassify)
	}

	// 文件分析
	{
		ai.POST("/analyze", api.analyzeFile)
		ai.GET("/analyze/:sha1", api.getAnalysis)
		ai.POST("/batch/analyze", api.batchAnalyze)
	}

	// 搜索功能
	if api.config.EnableSearch {
		{
			ai.POST("/search/tags", api.searchByTags)
			ai.GET("/search/similar/:sha1", api.getSimilarFiles)
		}
	}

	// 数据洞察
	if api.config.EnableInsights {
		{
			ai.GET("/insights", api.getInsights)
			ai.GET("/insights/storage", api.getStorageInsights)
			ai.GET("/insights/activity", api.getActivityInsights)
		}
	}

	// 服务状态
	{
		ai.GET("/health", api.health)
		ai.GET("/stats", api.getStats)
		ai.GET("/config", api.getConfig)
		ai.PUT("/config", api.updateConfig)
	}
}

// classifyFile 分类单个文件
// @Summary 分类文件
// @Description 使用AI对文件进行智能分类和标签生成
// @Tags AI Classification
// @Accept json
// @Produce json
// @Param request body ClassificationRequest true "分类请求"
// @Success 200 {object} ClassificationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/ai/classify [post]
func (api *API) classifyFile(c *gin.Context) {
	var req ClassificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// 这里需要从实际的上传文件中获取multipart.FileHeader
	// 目前模拟处理
	result := &ClassificationResult{
		ContentType:  ContentTypeDocument,
		Category:     "文档",
		Subcategory:  "文本文档",
		Confidence:   0.9,
		Tags:         []string{"文档", "文本"},
		Metadata:     map[string]interface{}{"filename": req.Filename},
		ProcessedAt:  time.Now(),
		ModelVersion: "v1.0.0",
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// getClassification 获取文件分类信息
// @Summary 获取文件分类
// @Description 获取指定文件的AI分类信息
// @Tags AI Classification
// @Accept json
// @Produce json
// @Param sha1 path string true "文件SHA1哈希"
// @Success 200 {object} ClassificationResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/ai/classify/{sha1} [get]
func (api *API) getClassification(c *gin.Context) {
	sha1 := c.Param("sha1")
	if sha1 == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "SHA1 hash is required",
		})
		return
	}

	// 这里应该从元数据中获取分类信息
	// 目前返回模拟结果
	result := &ClassificationResult{
		ContentType:  ContentTypeDocument,
		Category:     "文档",
		Subcategory:  "PDF文档",
		Confidence:   0.95,
		Tags:         []string{"文档", "PDF", "重要"},
		ProcessedAt:  time.Now(),
		ModelVersion: "v1.0.0",
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// batchClassify 批量分类文件
// @Summary 批量分类文件
// @Description 批量对多个文件进行AI分类
// @Tags AI Classification
// @Accept json
// @Produce json
// @Param request body BatchClassificationRequest true "批量分类请求"
// @Success 200 {object} BatchClassificationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/ai/batch/classify [post]
func (api *API) batchClassify(c *gin.Context) {
	if !api.config.EnableBatch {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Batch processing is disabled",
		})
		return
	}

	var req BatchClassificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	if len(req.Files) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Too many files in batch request (max: 100)",
		})
		return
	}

	// 这里应该执行真正的批量分类
	// 目前返回模拟结果
	results := make(map[string]*ClassificationResult)
	for _, file := range req.Files {
		results[file.SHA1] = &ClassificationResult{
			ContentType:  ContentTypeDocument,
			Category:     "文档",
			Confidence:   0.85,
			Tags:         []string{"批量处理", "文档"},
			ProcessedAt:  time.Now(),
			ModelVersion: "v1.0.0",
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"processed": len(results),
			"total":     len(req.Files),
			"results":   results,
		},
	})
}

// analyzeFile 分析文件
// @Summary 分析文件
// @Description 对文件进行深度AI分析
// @Tags AI Analysis
// @Accept json
// @Produce json
// @Param request body AnalysisRequest true "分析请求"
// @Success 200 {object} AnalysisResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/ai/analyze [post]
func (api *API) analyzeFile(c *gin.Context) {
	var req AnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// 这里应该执行真正的文件分析
	// 目前返回模拟结果
	result := &AnalysisResult{
		FileInfo: &FileInfo{
			Name:         req.Filename,
			Size:         req.Size,
			MimeType:     req.MimeType,
			LastModified: time.Now(),
		},
		Classification: &ClassificationResult{
			ContentType:  ContentTypeDocument,
			Category:     "文档",
			Confidence:   0.9,
			Tags:         []string{"分析", "文档"},
			ProcessedAt:  time.Now(),
			ModelVersion: "v1.0.0",
		},
		Content: &ContentInfo{
			Type:      "text",
			Encoding:  "utf-8",
			Language:  "zh",
			WordCount: 1000,
			LineCount: 50,
		},
		Keywords:       []string{"AI", "分析", "文档", "智能"},
		AnalyzedAt:     time.Now(),
		ProcessingTime: 500 * time.Millisecond,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// getAnalysis 获取文件分析信息
// @Summary 获取文件分析
// @Description 获取指定文件的深度分析信息
// @Tags AI Analysis
// @Accept json
// @Produce json
// @Param sha1 path string true "文件SHA1哈希"
// @Success 200 {object} AnalysisResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/ai/analyze/{sha1} [get]
func (api *API) getAnalysis(c *gin.Context) {
	sha1 := c.Param("sha1")
	if sha1 == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "SHA1 hash is required",
		})
		return
	}

	// 这里应该从存储中获取分析信息
	// 目前返回模拟结果
	result := &AnalysisResult{
		FileInfo: &FileInfo{
			Name:         "example.txt",
			Size:         1024,
			MimeType:     "text/plain",
			LastModified: time.Now(),
		},
		Classification: &ClassificationResult{
			ContentType:  ContentTypeDocument,
			Category:     "文档",
			Confidence:   0.95,
			Tags:         []string{"文档", "文本"},
			ProcessedAt:  time.Now(),
			ModelVersion: "v1.0.0",
		},
		Content: &ContentInfo{
			Type:      "text",
			Encoding:  "utf-8",
			Language:  "zh",
			WordCount: 200,
			LineCount: 10,
		},
		Keywords:       []string{"示例", "文本", "文档"},
		AnalyzedAt:     time.Now(),
		ProcessingTime: 200 * time.Millisecond,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// batchAnalyze 批量分析文件
// @Summary 批量分析文件
// @Description 批量对多个文件进行深度AI分析
// @Tags AI Analysis
// @Accept json
// @Produce json
// @Param request body BatchAnalysisRequest true "批量分析请求"
// @Success 200 {object} BatchAnalysisResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/ai/batch/analyze [post]
func (api *API) batchAnalyze(c *gin.Context) {
	if !api.config.EnableBatch {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Batch processing is disabled",
		})
		return
	}

	var req BatchAnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	if len(req.Files) > 50 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Too many files in batch request (max: 50)",
		})
		return
	}

	// 这里应该执行真正的批量分析
	// 目前返回模拟结果
	results := make(map[string]*AnalysisResult)
	for _, file := range req.Files {
		results[file.SHA1] = &AnalysisResult{
			FileInfo: &FileInfo{
				Name:     file.Filename,
				Size:     file.Size,
				MimeType: file.MimeType,
			},
			Classification: &ClassificationResult{
				ContentType:  ContentTypeDocument,
				Category:     "文档",
				Confidence:   0.8,
				Tags:         []string{"批量分析", "文档"},
				ProcessedAt:  time.Now(),
				ModelVersion: "v1.0.0",
			},
			AnalyzedAt:     time.Now(),
			ProcessingTime: 300 * time.Millisecond,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"processed": len(results),
			"total":     len(req.Files),
			"results":   results,
		},
	})
}

// searchByTags 根据标签搜索文件
// @Summary 标签搜索
// @Description 根据AI生成的标签搜索文件
// @Tags AI Search
// @Accept json
// @Produce json
// @Param request body SearchRequest true "搜索请求"
// @Success 200 {object} SearchResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/ai/search/tags [post]
func (api *API) searchByTags(c *gin.Context) {
	if !api.config.EnableSearch {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Search feature is disabled",
		})
		return
	}

	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	if req.Limit == 0 {
		req.Limit = 20
	}

	// 这里应该执行真正的标签搜索
	// 目前返回模拟结果
	files := []*types.FileMetadata{
		{
			SHA1:       "example_sha1_1",
			FileName:   "document1.pdf",
			Size:       1024000,
			UploadedAt: time.Now().Add(-24 * time.Hour),
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"files":  files,
			"total":  len(files),
			"limit":  req.Limit,
			"offset": req.Offset,
			"tags":   req.Tags,
		},
	})
}

// getSimilarFiles 获取相似文件
// @Summary 获取相似文件
// @Description 基于AI分析获取相似文件推荐
// @Tags AI Search
// @Accept json
// @Produce json
// @Param sha1 path string true "文件SHA1哈希"
// @Param limit query int false "返回数量限制" default(10)
// @Success 200 {object} SimilarFilesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/ai/search/similar/{sha1} [get]
func (api *API) getSimilarFiles(c *gin.Context) {
	if !api.config.EnableSearch {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Search feature is disabled",
		})
		return
	}

	sha1 := c.Param("sha1")
	if sha1 == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "SHA1 hash is required",
		})
		return
	}

	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 20 {
		limit = 10
	}

	// 这里应该执行真正的相似性搜索
	// 目前返回模拟结果
	files := []*types.FileMetadata{
		{
			SHA1:       "similar_sha1_1",
			FileName:   "similar_document.pdf",
			Size:       800000,
			UploadedAt: time.Now().Add(-12 * time.Hour),
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"source_sha1": sha1,
			"similar_files": files,
			"total":       len(files),
			"limit":       limit,
		},
	})
}

// getInsights 获取数据洞察
// @Summary 获取数据洞察
// @Description 获取基于AI分析的存储数据洞察
// @Tags AI Insights
// @Accept json
// @Produce json
// @Param timeRange query string false "时间范围" Enums(7d,30d,90d,1y) default(30d)
// @Success 200 {object} InsightsResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/ai/insights [get]
func (api *API) getInsights(c *gin.Context) {
	if !api.config.EnableInsights {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Insights feature is disabled",
		})
		return
	}

	timeRange := c.DefaultQuery("timeRange", "30d")

	// 这里应该执行真正的洞察分析
	// 目前返回模拟结果
	insights := &InsightsResult{
		TimeRange:  timeRange,
		TotalFiles: 1500,
		Categories: map[string]int{
			"文档": 600,
			"图片": 450,
			"视频": 300,
			"其他": 150,
		},
		Tags: map[string]int{
			"重要":   300,
			"工作":   250,
			"个人":   200,
			"项目":   150,
			"学习":   100,
		},
		StorageUsage: StorageUsageStats{
			TotalSize:  10 * 1024 * 1024 * 1024, // 10GB
			AverageSize: 5 * 1024 * 1024,       // 5MB
			GrowthRate: 0.15,                   // 15%
		},
		GeneratedAt: time.Now(),
		CacheUntil:  time.Now().Add(5 * time.Minute),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    insights,
	})
}

// getStorageInsights 获取存储洞察
// @Summary 获取存储洞察
// @Description 获取详细的存储使用分析
// @Tags AI Insights
// @Accept json
// @Produce json
// @Success 200 {object} StorageInsightsResponse
// @Router /api/v1/ai/insights/storage [get]
func (api *API) getStorageInsights(c *gin.Context) {
	if !api.config.EnableInsights {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Insights feature is disabled",
		})
		return
	}

	// 这里应该执行真正的存储洞察分析
	// 目前返回模拟结果
	storageStats := StorageUsageStats{
		TotalSize: 15 * 1024 * 1024 * 1024, // 15GB
		ByCategory: map[string]int64{
			"文档": 6 * 1024 * 1024 * 1024,  // 6GB
			"图片": 5 * 1024 * 1024 * 1024,  // 5GB
			"视频": 3 * 1024 * 1024 * 1024,  // 3GB
			"其他": 1 * 1024 * 1024 * 1024,  // 1GB
		},
		ByFileType: map[string]int64{
			".pdf":  3 * 1024 * 1024 * 1024, // 3GB
			".jpg":  2 * 1024 * 1024 * 1024, // 2GB
			".mp4":  2 * 1024 * 1024 * 1024, // 2GB
			".docx": 1 * 1024 * 1024 * 1024, // 1GB
		},
		LargestFiles: []FileInfo{
			{Name: "video.mp4", Size: 500 * 1024 * 1024, MimeType: "video/mp4"},
			{Name: "archive.zip", Size: 200 * 1024 * 1024, MimeType: "application/zip"},
		},
		AverageSize: 8 * 1024 * 1024, // 8MB
		GrowthRate:  0.20,            // 20%
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    storageStats,
	})
}

// getActivityInsights 获取活动洞察
// @Summary 获取活动洞察
// @Description 获取用户活动和系统使用统计
// @Tags AI Insights
// @Accept json
// @Produce json
// @Success 200 {object} ActivityInsightsResponse
// @Router /api/v1/ai/insights/activity [get]
func (api *API) getActivityInsights(c *gin.Context) {
	if !api.config.EnableInsights {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Insights feature is disabled",
		})
		return
	}

	// 这里应该执行真正的活动洞察分析
	// 目前返回模拟结果
	activityStats := ActivityStats{
		UploadsPerDay: map[string]int{
			"2025-10-01": 45,
			"2025-10-02": 52,
			"2025-10-03": 38,
			"2025-10-04": 61,
			"2025-10-05": 49,
		},
		DownloadsPerDay: map[string]int{
			"2025-10-01": 120,
			"2025-10-02": 135,
			"2025-10-03": 98,
			"2025-10-04": 156,
			"2025-10-05": 142,
		},
		PeakHours:      []int{9, 10, 14, 15, 16},
		MostActiveTags: []string{"工作", "重要", "项目"},
		RecentActivity: []ActivityItem{
			{Type: "upload", SHA1: "file1", Filename: "report.pdf", Timestamp: time.Now().Add(-1 * time.Hour)},
			{Type: "upload", SHA1: "file2", Filename: "image.jpg", Timestamp: time.Now().Add(-2 * time.Hour)},
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    activityStats,
	})
}

// health 健康检查
// @Summary AI服务健康检查
// @Description 检查AI服务的运行状态
// @Tags AI Service
// @Accept json
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/ai/health [get]
func (api *API) health(c *gin.Context) {
	err := api.aiService.Health()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "AI service is unhealthy",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"status":    "healthy",
			"timestamp": time.Now(),
			"version":   "v1.0.0",
		},
	})
}

// getStats 获取服务统计
// @Summary 获取服务统计
// @Description 获取AI服务的运行统计信息
// @Tags AI Service
// @Accept json
// @Produce json
// @Success 200 {object} StatsResponse
// @Router /api/v1/ai/stats [get]
func (api *API) getStats(c *gin.Context) {
	stats := api.aiService.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// getConfig 获取配置
// @Summary 获取AI配置
// @Description 获取当前AI服务的配置信息
// @Tags AI Service
// @Accept json
// @Produce json
// @Success 200 {object} ConfigResponse
// @Router /api/v1/ai/config [get]
func (api *API) getConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    api.config,
	})
}

// updateConfig 更新配置
// @Summary 更新AI配置
// @Description 更新AI服务的配置
// @Tags AI Service
// @Accept json
// @Produce json
// @Param config body APIConfig true "配置信息"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/ai/config [put]
func (api *API) updateConfig(c *gin.Context) {
	var config APIConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid configuration format",
			"details": err.Error(),
		})
		return
	}

	// 这里应该执行配置更新逻辑
	// 目前只是更新内存中的配置
	api.config = &config

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Configuration updated successfully",
	})
}

// 响应结构体定义

// ClassificationResponse 分类响应
type ClassificationResponse struct {
	Success bool                 `json:"success"`
	Data    *ClassificationResult `json:"data"`
}

// AnalysisResponse 分析响应
type AnalysisResponse struct {
	Success bool            `json:"success"`
	Data    *AnalysisResult `json:"data"`
}

// BatchClassificationResponse 批量分类响应
type BatchClassificationResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Processed int                           `json:"processed"`
		Total     int                           `json:"total"`
		Results   map[string]*ClassificationResult `json:"results"`
	} `json:"data"`
}

// BatchAnalysisResponse 批量分析响应
type BatchAnalysisResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Processed int                      `json:"processed"`
		Total     int                      `json:"total"`
		Results   map[string]*AnalysisResult `json:"results"`
	} `json:"data"`
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Files  []*types.FileMetadata `json:"files"`
		Total  int                   `json:"total"`
		Limit  int                   `json:"limit"`
		Offset int                   `json:"offset"`
		Tags   []string              `json:"tags"`
	} `json:"data"`
}

// SimilarFilesResponse 相似文件响应
type SimilarFilesResponse struct {
	Success bool `json:"success"`
	Data    struct {
		SourceSHA1   string                `json:"source_sha1"`
		SimilarFiles []*types.FileMetadata `json:"similar_files"`
		Total        int                   `json:"total"`
		Limit        int                   `json:"limit"`
	} `json:"data"`
}

// InsightsResponse 洞察响应
type InsightsResponse struct {
	Success bool           `json:"success"`
	Data    *InsightsResult `json:"data"`
}

// StorageInsightsResponse 存储洞察响应
type StorageInsightsResponse struct {
	Success bool               `json:"success"`
	Data    StorageUsageStats   `json:"data"`
}

// ActivityInsightsResponse 活动洞察响应
type ActivityInsightsResponse struct {
	Success bool         `json:"success"`
	Data    ActivityStats `json:"data"`
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Status    string    `json:"status"`
		Timestamp time.Time `json:"timestamp"`
		Version   string    `json:"version"`
	} `json:"data"`
}

// StatsResponse 统计响应
type StatsResponse struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data"`
}

// ConfigResponse 配置响应
type ConfigResponse struct {
	Success bool      `json:"success"`
	Data    *APIConfig `json:"data"`
}

// SuccessResponse 成功响应
type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}