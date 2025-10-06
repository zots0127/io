package search

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/zots0127/io/pkg/ai"
	"github.com/zots0127/io/pkg/metadata/repository"
	"github.com/zots0127/io/pkg/types"
)

// SearchService 搜索服务接口
type SearchService interface {
	Search(ctx context.Context, query *SearchQuery) (*SearchResult, error)
	AdvancedSearch(ctx context.Context, query *AdvancedSearchQuery) (*SearchResult, error)
	Suggest(ctx context.Context, partial string, limit int) ([]string, error)
	GetSimilarFiles(ctx context.Context, sha1 string, limit int) ([]*SimilarFile, error)
	GetPopularFiles(ctx context.Context, limit int) ([]*types.FileMetadata, error)
	GetRecentFiles(ctx context.Context, limit int) ([]*types.FileMetadata, error)
	GetTrendingTags(ctx context.Context, limit int) ([]TagTrend, error)
	GetSearchAnalytics(ctx context.Context, timeRange string) (*SearchAnalytics, error)
	IndexFile(ctx context.Context, metadata *types.FileMetadata) error
	RemoveFromIndex(ctx context.Context, sha1 string) error
	RebuildIndex(ctx context.Context) error
	GetStats() map[string]interface{}
	Health() error
}

// SearchServiceImpl 搜索服务实现
type SearchServiceImpl struct {
	searchEngine  *SearchEngine
	aiService     ai.AIService
	metadataRepo  *repository.MetadataRepository
	config        *SearchServiceConfig
	logger        *log.Logger
	searchHistory *SearchHistory
	analytics     *SearchAnalytics
	mu            sync.RWMutex
}

// SearchServiceConfig 搜索服务配置
type SearchServiceConfig struct {
	EnableRealTimeIndexing bool          `json:"enable_real_time_indexing" yaml:"enable_real_time_indexing"`
	EnableAnalytics       bool          `json:"enable_analytics" yaml:"enable_analytics"`
	EnableHistory         bool          `json:"enable_history" yaml:"enable_history"`
	EnableTrending        bool          `json:"enable_trending" yaml:"enable_trending"`
	AnalyticsRetention    time.Duration `json:"analytics_retention" yaml:"analytics_retention"`
	HistoryRetention      time.Duration `json:"history_retention" yaml:"history_retention"`
	TrendingWindow        time.Duration `json:"trending_window" yaml:"trending_window"`
	MaxSearchHistory      int           `json:"max_search_history" yaml:"max_search_history"`
	EnablePersonalization bool          `json:"enable_personalization" yaml:"enable_personalization"`
	EnableAIOptimization  bool          `json:"enable_ai_optimization" yaml:"enable_ai_optimization"`
}

// AdvancedSearchQuery 高级搜索查询
type AdvancedSearchQuery struct {
	*SearchQuery
	ContentQuery    string                 `json:"content_query"`
	SemanticQuery   string                 `json:"semantic_query"`
	Exclusions      []string               `json:"exclusions"`
	RequiredTerms   []string               `json:"required_terms"`
	ProximitySearch *ProximitySearch       `json:"proximity_search"`
	NaturalLanguage bool                   `json:"natural_language"`
	UserPreferences  map[string]interface{} `json:"user_preferences"`
	Weights         map[string]float64     `json:"weights"`
}

// ProximitySearch 邻近搜索
type ProximitySearch struct {
	Terms      []string `json:"terms"`
	MaxDistance int     `json:"max_distance"`
	InOrder    bool     `json:"in_order"`
}

// TagTrend 标签趋势
type TagTrend struct {
	Tag         string    `json:"tag"`
	Count       int       `json:"count"`
	GrowthRate  float64   `json:"growth_rate"`
	LastUsed    time.Time `json:"last_used"`
	Trending    bool      `json:"trending"`
}

// SearchAnalytics 搜索分析
type SearchAnalytics struct {
	TimeRange       string                   `json:"time_range"`
	TotalSearches   int                      `json:"total_searches"`
	UniqueQueries   int                      `json:"unique_queries"`
	AverageResults  float64                  `json:"average_results"`
	PopularQueries  []QueryStat             `json:"popular_queries"`
	SearchTrends    []SearchTrend            `json:"search_trends"`
	UserEngagement  map[string]interface{}   `json:"user_engagement"`
	Performance     map[string]interface{}   `json:"performance"`
	Insights        []string                 `json:"insights"`
	GeneratedAt     time.Time                `json:"generated_at"`
}

// QueryStat 查询统计
type QueryStat struct {
	Query      string    `json:"query"`
	Count      int       `json:"count"`
	LastSearch time.Time `json:"last_search"`
	AvgResults float64   `json:"avg_results"`
}

// SearchTrend 搜索趋势
type SearchTrend struct {
	Date       string `json:"date"`
	Searches   int    `json:"searches"`
	UniqueUsers int   `json:"unique_users"`
}

// SearchHistory 搜索历史
type SearchHistory struct {
	entries map[string][]*HistoryEntry
	mu      sync.RWMutex
	maxSize int
}

// HistoryEntry 历史记录条目
type HistoryEntry struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"user_id"`
	Query       *SearchQuery           `json:"query"`
	Results     int                    `json:"results"`
	Clicked     []string               `json:"clicked"` // 点击的文件SHA1
	Duration    time.Duration          `json:"duration"`
	Timestamp   time.Time              `json:"timestamp"`
	UserAgent   string                 `json:"user_agent"`
	IPAddress   string                 `json:"ip_address"`
	SessionID   string                 `json:"session_id"`
	Preferences map[string]interface{} `json:"preferences"`
}

// NewSearchService 创建搜索服务
func NewSearchService(aiService ai.AIService, metadataRepo *repository.MetadataRepository, config *SearchServiceConfig) SearchService {
	if config == nil {
		config = &SearchServiceConfig{
			EnableRealTimeIndexing: true,
			EnableAnalytics:        true,
			EnableHistory:          true,
			EnableTrending:         true,
			AnalyticsRetention:     30 * 24 * time.Hour, // 30 days
			HistoryRetention:       7 * 24 * time.Hour,  // 7 days
			TrendingWindow:         24 * time.Hour,     // 24 hours
			MaxSearchHistory:       1000,
			EnablePersonalization:  false,
			EnableAIOptimization:   true,
		}
	}

	// 创建搜索引擎配置
	searchConfig := &SearchConfig{
		EnableFullTextSearch: true,
		EnableSemanticSearch: config.EnableAIOptimization,
		EnableFuzzySearch:    true,
		EnableAutoComplete:   true,
		MaxResults:           100,
		QueryTimeout:         30 * time.Second,
		CacheExpiration:      5 * time.Minute,
		MinQueryLength:        2,
		SimilarityThreshold:  0.7,
		BoostRecentFiles:     true,
		BoostPopularFiles:    false,
	}

	searchEngine := NewSearchEngine(aiService, metadataRepo, searchConfig)

	service := &SearchServiceImpl{
		searchEngine:  searchEngine,
		aiService:     aiService,
		metadataRepo:  metadataRepo,
		config:        config,
		logger:        log.New(log.Writer(), "[SEARCH-SVC] ", log.LstdFlags),
		searchHistory: NewSearchHistory(config.MaxSearchHistory),
		analytics: &SearchAnalytics{
			GeneratedAt: time.Now(),
		},
	}

	// 启动后台任务
	go service.startBackgroundTasks()

	return service
}

// Search 执行搜索
func (s *SearchServiceImpl) Search(ctx context.Context, query *SearchQuery) (*SearchResult, error) {
	startTime := time.Now()

	// 记录搜索历史
	if s.config.EnableHistory {
		s.recordSearchHistory(ctx, query, 0) // 结果数稍后更新
	}

	// 执行搜索
	result, err := s.searchEngine.Search(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// 更新搜索历史
	if s.config.EnableHistory {
		s.updateSearchHistory(ctx, query, len(result.Files))
	}

	// 记录分析数据
	if s.config.EnableAnalytics {
		s.recordAnalytics(ctx, query, result)
	}

	// AI优化（如果启用）
	if s.config.EnableAIOptimization && s.aiService != nil {
		result = s.applyAIOptimization(ctx, query, result)
	}

	s.logger.Printf("Search completed in %v: %d results", time.Since(startTime), result.Total)
	return result, nil
}

// AdvancedSearch 高级搜索
func (s *SearchServiceImpl) AdvancedSearch(ctx context.Context, query *AdvancedSearchQuery) (*SearchResult, error) {
	// 转换为标准搜索查询
	standardQuery := query.SearchQuery

	// 应用高级搜索逻辑
	if query.ContentQuery != "" {
		standardQuery.IncludeContent = true
	}

	if query.SemanticQuery != "" && s.config.EnableAIOptimization {
		// 使用AI进行语义搜索
		similarFiles, err := s.aiService.GetSimilarFiles(ctx, query.SemanticQuery, 50)
		if err == nil {
			// 将语义搜索结果合并到查询中
			standardQuery.Filters["semantic_results"] = similarFiles
		}
	}

	// 应用权重
	if len(query.Weights) > 0 {
		standardQuery.Filters["search_weights"] = query.Weights
	}

	// 执行搜索
	result, err := s.Search(ctx, standardQuery)
	if err != nil {
		return nil, err
	}

	// 应用排除项
	if len(query.Exclusions) > 0 {
		result.Files = s.applyExclusions(result.Files, query.Exclusions)
	}

	// 应用必需词
	if len(query.RequiredTerms) > 0 {
		result.Files = s.applyRequiredTerms(result.Files, query.RequiredTerms)
	}

	return result, nil
}

// Suggest 搜索建议
func (s *SearchServiceImpl) Suggest(ctx context.Context, partial string, limit int) ([]string, error) {
	if len(partial) < 2 {
		return []string{}, nil
	}

	// 从搜索历史中获取建议
	historySuggestions := s.getHistorySuggestions(partial, limit/2)

	// 从搜索引擎获取建议
	engineSuggestions := s.searchEngine.GenerateSuggestions(partial)

	// 合并和去重
	allSuggestions := append(historySuggestions, engineSuggestions...)
	uniqueSuggestions := make([]string, 0, limit)
	seen := make(map[string]bool)

	for _, suggestion := range allSuggestions {
		if !seen[suggestion] && len(uniqueSuggestions) < limit {
			seen[suggestion] = true
			uniqueSuggestions = append(uniqueSuggestions, suggestion)
		}
	}

	return uniqueSuggestions, nil
}

// GetSimilarFiles 获取相似文件
func (s *SearchServiceImpl) GetSimilarFiles(ctx context.Context, sha1 string, limit int) ([]*SimilarFile, error) {
	return s.searchEngine.GetSimilarFiles(ctx, sha1, limit)
}

// GetPopularFiles 获取热门文件
func (s *SearchServiceImpl) GetPopularFiles(ctx context.Context, limit int) ([]*types.FileMetadata, error) {
	query := &SearchQuery{
		SortBy:    SortByDownloads,
		SortOrder: SortOrderDesc,
		Limit:     limit,
	}

	result, err := s.searchEngine.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	var files []*types.FileMetadata
	for _, searchFile := range result.Files {
		files = append(files, searchFile.FileMetadata)
	}

	return files, nil
}

// GetRecentFiles 获取最新文件
func (s *SearchServiceImpl) GetRecentFiles(ctx context.Context, limit int) ([]*types.FileMetadata, error) {
	query := &SearchQuery{
		SortBy:    SortByDate,
		SortOrder: SortOrderDesc,
		Limit:     limit,
	}

	result, err := s.searchEngine.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	var files []*types.FileMetadata
	for _, searchFile := range result.Files {
		files = append(files, searchFile.FileMetadata)
	}

	return files, nil
}

// GetTrendingTags 获取趋势标签
func (s *SearchServiceImpl) GetTrendingTags(ctx context.Context, limit int) ([]TagTrend, error) {
	if !s.config.EnableTrending {
		return []TagTrend{}, nil
	}

	// 从分析数据中计算趋势标签
	tags := make(map[string]*TagTrend)

	// 分析最近的搜索历史
	s.searchHistory.mu.RLock()
	for _, entries := range s.searchHistory.entries {
		for _, entry := range entries {
			if time.Since(entry.Timestamp) > s.config.TrendingWindow {
				continue
			}

			for _, tag := range entry.Query.Tags {
				if tags[tag] == nil {
					tags[tag] = &TagTrend{
						Tag:      tag,
						Count:    0,
						LastUsed: entry.Timestamp,
					}
				}
				tags[tag].Count++
				if entry.Timestamp.After(tags[tag].LastUsed) {
					tags[tag].LastUsed = entry.Timestamp
				}
			}
		}
	}
	s.searchHistory.mu.RUnlock()

	// 转换为切片并排序
	var trends []TagTrend
	for _, trend := range tags {
		// 计算增长率（简化实现）
		trend.GrowthRate = float64(trend.Count) / 24.0 // 每小时平均
		trend.Trending = trend.Count > 5 // 简单的趋势判断
		trends = append(trends, *trend)
	}

	// 按数量排序
	for i := 0; i < len(trends)-1; i++ {
		for j := i + 1; j < len(trends); j++ {
			if trends[i].Count < trends[j].Count {
				trends[i], trends[j] = trends[j], trends[i]
			}
		}
	}

	// 限制结果数量
	if len(trends) > limit {
		trends = trends[:limit]
	}

	return trends, nil
}

// GetSearchAnalytics 获取搜索分析
func (s *SearchServiceImpl) GetSearchAnalytics(ctx context.Context, timeRange string) (*SearchAnalytics, error) {
	if !s.config.EnableAnalytics {
		return &SearchAnalytics{
			TimeRange:   timeRange,
			GeneratedAt: time.Now(),
		}, nil
	}

	// 构建分析报告
	analytics := &SearchAnalytics{
		TimeRange:   timeRange,
		GeneratedAt: time.Now(),
	}

	// 统计搜索查询
	s.searchHistory.mu.RLock()
	queryStats := make(map[string]*QueryStat)
	totalSearches := 0
	totalResults := 0

	for _, entries := range s.searchHistory.entries {
		for _, entry := range entries {
			if !s.isInTimeRange(entry.Timestamp, timeRange) {
				continue
			}

			totalSearches++
			totalResults += entry.Results

			queryKey := entry.Query.Query
			if stat, exists := queryStats[queryKey]; exists {
				stat.Count++
				stat.AvgResults = (stat.AvgResults + float64(entry.Results)) / 2
				if entry.Timestamp.After(stat.LastSearch) {
					stat.LastSearch = entry.Timestamp
				}
			} else {
				queryStats[queryKey] = &QueryStat{
					Query:      queryKey,
					Count:      1,
					LastSearch: entry.Timestamp,
					AvgResults: float64(entry.Results),
				}
			}
		}
	}
	s.searchHistory.mu.RUnlock()

	analytics.TotalSearches = totalSearches
	analytics.UniqueQueries = len(queryStats)
	if totalSearches > 0 {
		analytics.AverageResults = float64(totalResults) / float64(totalSearches)
	}

	// 热门查询
	for _, stat := range queryStats {
		analytics.PopularQueries = append(analytics.PopularQueries, *stat)
	}

	// 排序热门查询
	for i := 0; i < len(analytics.PopularQueries)-1; i++ {
		for j := i + 1; j < len(analytics.PopularQueries); j++ {
			if analytics.PopularQueries[i].Count < analytics.PopularQueries[j].Count {
				analytics.PopularQueries[i], analytics.PopularQueries[j] = analytics.PopularQueries[j], analytics.PopularQueries[i]
			}
		}
	}

	// 限制热门查询数量
	if len(analytics.PopularQueries) > 10 {
		analytics.PopularQueries = analytics.PopularQueries[:10]
	}

	// 生成洞察
	analytics.Insights = s.generateInsights(analytics)

	return analytics, nil
}

// IndexFile 索引文件
func (s *SearchServiceImpl) IndexFile(ctx context.Context, metadata *types.FileMetadata) error {
	if s.config.EnableRealTimeIndexing {
		s.searchEngine.indexFile(metadata)
	}
	return nil
}

// RemoveFromIndex 从索引中移除文件
func (s *SearchServiceImpl) RemoveFromIndex(ctx context.Context, sha1 string) error {
	// 实现从索引中移除文件
	s.searchEngine.index.mu.Lock()
	defer s.searchEngine.index.mu.Unlock()

	// 从倒排索引中移除文件
	for term, termInfo := range s.searchEngine.index.terms {
		if posting, exists := termInfo.Postings[sha1]; exists {
			delete(termInfo.Postings, sha1)
			termInfo.DF--
			if termInfo.DF == 0 {
				delete(s.searchEngine.index.terms, term)
			}
			_ = posting // 避免未使用变量警告
		}
	}

	return nil
}

// RebuildIndex 重建索引
func (s *SearchServiceImpl) RebuildIndex(ctx context.Context) error {
	return s.searchEngine.RebuildIndex()
}

// GetStats 获取统计信息
func (s *SearchServiceImpl) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := s.searchEngine.GetStats()
	stats["service_config"] = map[string]interface{}{
		"real_time_indexing": s.config.EnableRealTimeIndexing,
		"analytics":          s.config.EnableAnalytics,
		"history":            s.config.EnableHistory,
		"trending":           s.config.EnableTrending,
		"ai_optimization":    s.config.EnableAIOptimization,
	}

	stats["history_size"] = s.getHistorySize()
	stats["last_analytics"] = s.analytics.GeneratedAt

	return stats
}

// Health 健康检查
func (s *SearchServiceImpl) Health() error {
	if s.searchEngine == nil {
		return fmt.Errorf("search engine not initialized")
	}

	if s.metadataRepo == nil {
		return fmt.Errorf("metadata repository not available")
	}

	return nil
}

// 辅助方法实现

// NewSearchHistory 创建搜索历史
func NewSearchHistory(maxSize int) *SearchHistory {
	return &SearchHistory{
		entries: make(map[string][]*HistoryEntry),
		maxSize: maxSize,
	}
}

// recordSearchHistory 记录搜索历史
func (s *SearchServiceImpl) recordSearchHistory(ctx context.Context, query *SearchQuery, results int) {
	// 简化实现，实际应该从上下文中获取用户信息
	userID := "anonymous"
	sessionID := "session"

	entry := &HistoryEntry{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		UserID:    userID,
		Query:     query,
		Results:   results,
		Timestamp: time.Now(),
		SessionID: sessionID,
	}

	s.searchHistory.mu.Lock()
	defer s.searchHistory.mu.Unlock()

	if s.searchHistory.entries[userID] == nil {
		s.searchHistory.entries[userID] = []*HistoryEntry{}
	}

	s.searchHistory.entries[userID] = append(s.searchHistory.entries[userID], entry)

	// 限制历史记录数量
	if len(s.searchHistory.entries[userID]) > s.config.MaxSearchHistory {
		s.searchHistory.entries[userID] = s.searchHistory.entries[userID][1:]
	}
}

// updateSearchHistory 更新搜索历史
func (s *SearchServiceImpl) updateSearchHistory(ctx context.Context, query *SearchQuery, results int) {
	// 简化实现
}

// recordAnalytics 记录分析数据
func (s *SearchServiceImpl) recordAnalytics(ctx context.Context, query *SearchQuery, result *SearchResult) {
	// 简化实现
}

// applyAIOptimization 应用AI优化
func (s *SearchServiceImpl) applyAIOptimization(ctx context.Context, query *SearchQuery, result *SearchResult) *SearchResult {
	// 使用AI服务优化搜索结果
	if s.aiService == nil {
		return result
	}

	// 这里可以实现更复杂的AI优化逻辑
	// 例如：基于用户行为重新排序结果、个性化推荐等

	return result
}

// applyExclusions 应用排除项
func (s *SearchServiceImpl) applyExclusions(files []*SearchResultFile, exclusions []string) []*SearchResultFile {
	var filtered []*SearchResultFile
	for _, file := range files {
		shouldExclude := false
		for _, exclusion := range exclusions {
			if contains(file.FileName, exclusion) || containsAnyTag(file.Tags, exclusion) {
				shouldExclude = true
				break
			}
		}
		if !shouldExclude {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

// applyRequiredTerms 应用必需词
func (s *SearchServiceImpl) applyRequiredTerms(files []*SearchResultFile, requiredTerms []string) []*SearchResultFile {
	var filtered []*SearchResultFile
	for _, file := range files {
		hasAllTerms := true
		for _, term := range requiredTerms {
			if !contains(file.FileName, term) && !containsAnyTag(file.Tags, term) {
				hasAllTerms = false
				break
			}
		}
		if hasAllTerms {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

// getHistorySuggestions 从历史中获取建议
func (s *SearchServiceImpl) getHistorySuggestions(partial string, limit int) []string {
	s.searchHistory.mu.RLock()
	defer s.searchHistory.mu.RUnlock()

	suggestions := []string{}
	seen := make(map[string]bool)

	for _, entries := range s.searchHistory.entries {
		for _, entry := range entries {
			if contains(entry.Query.Query, partial) && !seen[entry.Query.Query] {
				suggestions = append(suggestions, entry.Query.Query)
				seen[entry.Query.Query] = true
				if len(suggestions) >= limit {
					return suggestions
				}
			}
		}
	}

	return suggestions
}

// generateInsights 生成洞察
func (s *SearchServiceImpl) generateInsights(analytics *SearchAnalytics) []string {
	insights := []string{}

	if analytics.TotalSearches == 0 {
		insights = append(insights, "No search activity recorded")
		return insights
	}

	// 分析搜索模式
	if analytics.AverageResults > 50 {
		insights = append(insights, "Users are getting many results, consider improving query specificity")
	} else if analytics.AverageResults < 5 {
		insights = append(insights, "Users are getting few results, consider broadening search scope")
	}

	if len(analytics.PopularQueries) > 0 {
		topQuery := analytics.PopularQueries[0]
		insights = append(insights, fmt.Sprintf("Most popular query: '%s' (%d times)", topQuery.Query, topQuery.Count))
	}

	return insights
}

// isInTimeRange 检查时间是否在范围内
func (s *SearchServiceImpl) isInTimeRange(timestamp time.Time, timeRange string) bool {
	// 简化实现，实际应该解析timeRange参数
	return time.Since(timestamp) <= 24*time.Hour
}

// getHistorySize 获取历史记录大小
func (s *SearchServiceImpl) getHistorySize() int {
	s.searchHistory.mu.RLock()
	defer s.searchHistory.mu.RUnlock()

	total := 0
	for _, entries := range s.searchHistory.entries {
		total += len(entries)
	}
	return total
}

// startBackgroundTasks 启动后台任务
func (s *SearchServiceImpl) startBackgroundTasks() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		// 清理过期的历史记录
		if s.config.EnableHistory {
			s.cleanupExpiredHistory()
		}

		// 更新趋势数据
		if s.config.EnableTrending {
			s.updateTrendingData()
		}
	}
}

// cleanupExpiredHistory 清理过期历史记录
func (s *SearchServiceImpl) cleanupExpiredHistory() {
	cutoff := time.Now().Add(-s.config.HistoryRetention)

	s.searchHistory.mu.Lock()
	defer s.searchHistory.mu.Unlock()

	for userID, entries := range s.searchHistory.entries {
		var filtered []*HistoryEntry
		for _, entry := range entries {
			if entry.Timestamp.After(cutoff) {
				filtered = append(filtered, entry)
			}
		}
		s.searchHistory.entries[userID] = filtered
	}
}

// updateTrendingData 更新趋势数据
func (s *SearchServiceImpl) updateTrendingData() {
	// 简化实现
	s.logger.Println("Updating trending data...")
}

// 辅助函数
func contains(text, substr string) bool {
	return len(text) >= len(substr) &&
		(text == substr ||
		 len(text) > len(substr) &&
		 (text[:len(substr)] == substr ||
		  text[len(text)-len(substr):] == substr ||
		  func() bool {
			for i := 0; i <= len(text)-len(substr); i++ {
				if text[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		  }()))
}

func containsAnyTag(tags []string, text string) bool {
	for _, tag := range tags {
		if contains(tag, text) {
			return true
		}
	}
	return false
}