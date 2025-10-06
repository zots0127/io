package search

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zots0127/io/pkg/middleware"
	"github.com/zots0127/io/pkg/types"
)

// API 搜索API
type API struct {
	searchEngine *SearchEngine
	config       *APIConfig
}

// APIConfig API配置
type APIConfig struct {
	EnableRateLimit  bool     `json:"enable_rate_limit"`
	EnableCORS       bool     `json:"enable_cors"`
	EnableAuth       bool     `json:"enable_auth"`
	MaxRequestSize   int64    `json:"max_request_size"`
	AllowedOrigins   []string `json:"allowed_origins"`
	RateLimitPerMin  int      `json:"rate_limit_per_min"`
	BasePath         string   `json:"base_path"`
}

// NewAPI 创建搜索API
func NewAPI(searchEngine *SearchEngine, config *APIConfig) *API {
	if config == nil {
		config = &APIConfig{
			EnableRateLimit: false,
			EnableCORS:      true,
			EnableAuth:      false,
			MaxRequestSize:  10 * 1024 * 1024, // 10MB
			AllowedOrigins:  []string{"*"},
			RateLimitPerMin: 100,
			BasePath:        "/api/v1/search",
		}
	}

	return &API{
		searchEngine: searchEngine,
		config:       config,
	}
}

// RegisterRoutes 注册路由
func (api *API) RegisterRoutes(router *gin.Engine, middlewareConfig *middleware.Config) {
	basePath := api.config.BasePath
	search := router.Group(basePath)

	// 应用中间件
	if middlewareConfig.EnableAuth && api.config.EnableAuth {
		// search.Use(api.authMiddleware())
	}
	if middlewareConfig.EnableRateLimit {
		// search.Use(api.rateLimitMiddleware())
	}
	if api.config.EnableCORS {
		// search.Use(api.corsMiddleware())
	}

	// 搜索端点
	search.POST("/", api.search)
	search.GET("/suggest", api.suggest)
	search.GET("/facets", api.facets)
	search.GET("/popular", api.popular)
	search.GET("/recent", api.recent)
	search.GET("/similar/:sha1", api.similar)

	// 高级搜索
	search.POST("/advanced", api.advancedSearch)
	search.GET("/history", api.searchHistory)
	search.POST("/save", api.saveSearch)

	// 管理端点
	search.GET("/stats", api.getSearchStats)
	search.POST("/index/rebuild", api.rebuildIndex)
	search.GET("/index/status", api.getIndexStatus)
}

// searchRequest 搜索请求
type searchRequest struct {
	Query          string                 `json:"query" binding:"required,min=1"`
	Tags           []string               `json:"tags"`
	Categories     []string               `json:"categories"`
	FileTypes      []string               `json:"file_types"`
	SizeRange      *SizeRange             `json:"size_range"`
	DateRange      *DateRange             `json:"date_range"`
	SortBy         SortBy                 `json:"sort_by"`
	SortOrder      SortOrder              `json:"sort_order"`
	Filters        map[string]interface{} `json:"filters"`
	IncludeContent bool                   `json:"include_content"`
	IncludeSimilar bool                   `json:"include_similar"`
	Limit          int                    `json:"limit"`
	Offset         int                    `json:"offset"`
}

// searchResponse 搜索响应
type searchResponse struct {
	Success     bool           `json:"success"`
	Message     string         `json:"message,omitempty"`
	Data        *SearchResult  `json:"data,omitempty"`
	Query       *SearchQuery   `json:"query,omitempty"`
	Performance *Performance  `json:"performance,omitempty"`
}

// Performance 性能指标
type Performance struct {
	QueryTime    time.Duration `json:"query_time"`
	IndexTime    time.Duration `json:"index_time"`
	CacheHit     bool          `json:"cache_hit"`
	ResultsCount int           `json:"results_count"`
}

// search 执行搜索
func (api *API) search(c *gin.Context) {
	var req searchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// 构建搜索查询
	query := &SearchQuery{
		Query:          req.Query,
		Tags:           req.Tags,
		Categories:     req.Categories,
		FileTypes:      req.FileTypes,
		SizeRange:      req.SizeRange,
		DateRange:      req.DateRange,
		SortBy:         req.SortBy,
		SortOrder:      req.SortOrder,
		Filters:        req.Filters,
		IncludeContent: req.IncludeContent,
		IncludeSimilar: req.IncludeSimilar,
		Limit:          req.Limit,
		Offset:         req.Offset,
	}

	// 设置默认值
	if query.SortBy == "" {
		query.SortBy = SortByRelevance
	}
	if query.SortOrder == "" {
		query.SortOrder = SortOrderDesc
	}
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}

	// 执行搜索
	result, err := api.searchEngine.Search(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Search failed: " + err.Error(),
		})
		return
	}

	// 构建响应
	response := &searchResponse{
		Success: true,
		Data:    result,
		Query:   query,
		Performance: &Performance{
			QueryTime:    result.QueryTime,
			ResultsCount: len(result.Files),
		},
	}

	c.JSON(http.StatusOK, response)
}

// suggest 自动建议
func (api *API) suggest(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Query parameter 'q' is required",
		})
		return
	}

	// 生成建议
	suggestions := api.searchEngine.GenerateSuggestions(query)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"query":       query,
			"suggestions": suggestions,
		},
	})
}

// facets 获取搜索分面
func (api *API) facets(c *gin.Context) {
	// 获取查询参数
	text := c.Query("q")
	tags := c.QueryArray("tags")
	categories := c.QueryArray("categories")
	fileTypes := c.QueryArray("file_types")

	// 构建搜索查询
	searchQuery := &SearchQuery{
		Query:      text,
		Tags:       tags,
		Categories: categories,
		FileTypes:  fileTypes,
		Limit:      0, // 获取所有结果用于生成分面
	}

	// 执行搜索
	result, err := api.searchEngine.Search(c.Request.Context(), searchQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get facets: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"facets": result.Facets,
		},
	})
}

// popular 热门文件
func (api *API) popular(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// 构建热门搜索查询
	query := &SearchQuery{
		SortBy:    SortByDownloads,
		SortOrder: SortOrderDesc,
		Limit:     limit,
	}

	result, err := api.searchEngine.Search(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get popular files: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"files": result.Files,
			"total": result.Total,
		},
	})
}

// recent 最新文件
func (api *API) recent(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// 构建最新搜索查询
	query := &SearchQuery{
		SortBy:    SortByDate,
		SortOrder: SortOrderDesc,
		Limit:     limit,
	}

	result, err := api.searchEngine.Search(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get recent files: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"files": result.Files,
			"total": result.Total,
		},
	})
}

// similar 相似文件
func (api *API) similar(c *gin.Context) {
	sha1 := c.Param("sha1")
	if sha1 == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "SHA1 parameter is required",
		})
		return
	}

	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	// 使用AI服务获取相似文件
	// 注意：这里需要访问AI服务，可以通过searchEngine或者直接注入
	similarFiles, err := api.searchEngine.GetSimilarFiles(c.Request.Context(), sha1, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get similar files: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"sha1":     sha1,
			"similar":  similarFiles,
			"total":    len(similarFiles),
		},
	})
}

// advancedSearch 高级搜索
func (api *API) advancedSearch(c *gin.Context) {
	var req AdvancedSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// 转换为标准搜索查询
	query := req.ToSearchQuery()

	// 执行搜索
	result, err := api.searchEngine.Search(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Advanced search failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// searchHistory 搜索历史
func (api *API) searchHistory(c *gin.Context) {
	// 获取用户搜索历史
	// 这里可以实现用户认证和搜索历史记录

	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}

	// 模拟搜索历史
	history := []SearchHistoryItem{
		{
			Query:     "document",
			Timestamp: time.Now().Add(-1 * time.Hour),
			Results:   15,
		},
		{
			Query:     "image",
			Timestamp: time.Now().Add(-2 * time.Hour),
			Results:   8,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"history": history,
			"total":   len(history),
		},
	})
}

// saveSearch 保存搜索
func (api *API) saveSearch(c *gin.Context) {
	var req SaveSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// 保存搜索到用户收藏
	// 这里可以实现用户认证和搜索收藏功能

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Search saved successfully",
	})
}

// getSearchStats 获取搜索统计
func (api *API) getSearchStats(c *gin.Context) {
	stats := api.searchEngine.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// rebuildIndex 重建索引
func (api *API) rebuildIndex(c *gin.Context) {
	// 启动后台索引重建任务
	go func() {
		err := api.searchEngine.RebuildIndex()
		if err != nil {
			// 记录错误日志
			return
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Index rebuild started",
	})
}

// getIndexStatus 获取索引状态
func (api *API) getIndexStatus(c *gin.Context) {
	status := api.searchEngine.GetIndexStatus()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// 高级搜索相关结构体

// AdvancedSearchRequest 高级搜索请求
type AdvancedSearchRequest struct {
	Query            string                 `json:"query"`
	Tags             []string               `json:"tags"`
	Categories       []string               `json:"categories"`
	FileTypes        []string               `json:"file_types"`
	SizeRange        *SizeRange             `json:"size_range"`
	DateRange        *DateRange             `json:"date_range"`
	SortBy           SortBy                 `json:"sort_by"`
	SortOrder        SortOrder              `json:"sort_order"`
	Filters          map[string]interface{} `json:"filters"`
	IncludeContent   bool                   `json:"include_content"`
	IncludeSimilar   bool                   `json:"include_similar"`
	HighlightResults bool                   `json:"highlight_results"`
	Limit            int                    `json:"limit"`
	Offset           int                    `json:"offset"`
}

// ToSearchQuery 转换为搜索查询
func (req *AdvancedSearchRequest) ToSearchQuery() *SearchQuery {
	return &SearchQuery{
		Query:          req.Query,
		Tags:           req.Tags,
		Categories:     req.Categories,
		FileTypes:      req.FileTypes,
		SizeRange:      req.SizeRange,
		DateRange:      req.DateRange,
		SortBy:         req.SortBy,
		SortOrder:      req.SortOrder,
		Filters:        req.Filters,
		IncludeContent: req.IncludeContent,
		IncludeSimilar: req.IncludeSimilar,
		Limit:          req.Limit,
		Offset:         req.Offset,
	}
}

// SaveSearchRequest 保存搜索请求
type SaveSearchRequest struct {
	Query   string                 `json:"query"`
	Name    string                 `json:"name"`
	Filters map[string]interface{} `json:"filters"`
	Public  bool                   `json:"public"`
}

// SearchHistoryItem 搜索历史项
type SearchHistoryItem struct {
	Query     string    `json:"query"`
	Timestamp time.Time `json:"timestamp"`
	Results   int       `json:"results"`
}

// 为SearchEngine添加API需要的方法

// GenerateSuggestions 生成搜索建议
func (e *SearchEngine) GenerateSuggestions(query string) []string {
	// 实现搜索建议生成
	terms := e.tokenize(query)
	var suggestions []string

	// 基于索引中的词汇生成建议
	for _, term := range terms {
		similarTerms := e.index.FindSimilarTerms(term, 0.6)
		for _, similar := range similarTerms {
			if similar != term {
				suggestions = append(suggestions, similar)
			}
		}
	}

	// 去重并限制数量
	unique := make(map[string]bool)
	var result []string
	for _, s := range suggestions {
		if !unique[s] {
			unique[s] = true
			result = append(result, s)
			if len(result) >= 10 {
				break
			}
		}
	}

	return result
}

// GetSimilarFiles 获取相似文件
func (e *SearchEngine) GetSimilarFiles(ctx context.Context, sha1 string, limit int) ([]*SimilarFile, error) {
	if e.aiService == nil {
		return []*SimilarFile{}, nil
	}

	// 使用AI服务获取相似文件
	metadata, err := e.metadataRepo.GetMetadata(sha1)
	if err != nil {
		return nil, err
	}

	// 基于标签查找相似文件
	var similarFiles []*SimilarFile
	for _, tag := range metadata.Tags {
		query := &SearchQuery{
			Tags:       []string{tag},
			SortBy:     SortByRelevance,
			SortOrder:  SortOrderDesc,
			Limit:      limit + 1, // +1 来排除自己
		}

		result, err := e.Search(ctx, query)
		if err != nil {
			continue
		}

		for _, file := range result.Files {
			if file.SHA1 != sha1 { // 排除自己
				similarFiles = append(similarFiles, &SimilarFile{
					SHA1:     file.SHA1,
					Filename: file.FileName,
					Score:    file.Score,
					Reason:   fmt.Sprintf("Similar tag: %s", tag),
				})

				if len(similarFiles) >= limit {
					break
				}
			}
		}

		if len(similarFiles) >= limit {
			break
		}
	}

	return similarFiles, nil
}

// GetStats 获取搜索统计
func (e *SearchEngine) GetStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := map[string]interface{}{
		"index_size":       len(e.index.terms),
		"cache_size":       len(e.queryCache.entries),
		"cache_hit_rate":   e.calculateCacheHitRate(),
		"total_documents":  e.getTotalDocuments(),
		"last_indexed":     e.getLastIndexedTime(),
		"search_enabled":   true,
		"features": map[string]interface{}{
			"full_text_search":  e.config.EnableFullTextSearch,
			"semantic_search":   e.config.EnableSemanticSearch,
			"fuzzy_search":      e.config.EnableFuzzySearch,
			"auto_complete":     e.config.EnableAutoComplete,
		},
	}

	return stats
}

// RebuildIndex 重建索引
func (e *SearchEngine) RebuildIndex() error {
	e.logger.Println("Starting index rebuild...")

	// 清空现有索引
	e.index = NewInvertedIndex()

	// 重建索引
	e.buildIndex()

	e.logger.Println("Index rebuild completed")
	return nil
}

// GetIndexStatus 获取索引状态
func (e *SearchEngine) GetIndexStatus() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	status := map[string]interface{}{
		"status":          "ready",
		"total_terms":     len(e.index.terms),
		"total_documents": e.getTotalDocuments(),
		"last_updated":    time.Now(),
		"building":        false,
	}

	return status
}

// 辅助方法
func (e *SearchEngine) calculateCacheHitRate() float64 {
	e.queryCache.mu.RLock()
	defer e.queryCache.mu.RUnlock()

	totalHits := 0
	for _, entry := range e.queryCache.entries {
		totalHits += entry.HitCount
	}

	if len(e.queryCache.entries) == 0 {
		return 0.0
	}

	return float64(totalHits) / float64(len(e.queryCache.entries))
}

func (e *SearchEngine) getTotalDocuments() int {
	if e.metadataRepo == nil {
		return 0
	}

	files, err := e.metadataRepo.ListFiles(&types.MetadataFilter{})
	if err != nil {
		return 0
	}

	return len(files)
}

func (e *SearchEngine) getLastIndexedTime() time.Time {
	// 返回最后一次索引更新时间
	return time.Now()
}