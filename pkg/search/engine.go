package search

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zots0127/io/pkg/ai"
	"github.com/zots0127/io/pkg/metadata/repository"
	"github.com/zots0127/io/pkg/types"
)

// SearchEngine 智能搜索引擎
type SearchEngine struct {
	aiService     ai.AIService
	metadataRepo  *repository.MetadataRepository
	index         *InvertedIndex
	config        *SearchConfig
	logger        *log.Logger
	queryCache    *QueryCache
	mu            sync.RWMutex
}

// SearchConfig 搜索配置
type SearchConfig struct {
	EnableFullTextSearch  bool          `json:"enable_full_text_search" yaml:"enable_full_text_search"`
	EnableSemanticSearch  bool          `json:"enable_semantic_search" yaml:"enable_semantic_search"`
	EnableFuzzySearch     bool          `json:"enable_fuzzy_search" yaml:"enable_fuzzy_search"`
	EnableAutoComplete    bool          `json:"enable_auto_complete" yaml:"enable_auto_complete"`
	MaxResults            int           `json:"max_results" yaml:"max_results"`
	QueryTimeout          time.Duration `json:"query_timeout" yaml:"query_timeout"`
	CacheExpiration       time.Duration `json:"cache_expiration" yaml:"cache_expiration"`
	MinQueryLength        int           `json:"min_query_length" yaml:"min_query_length"`
	SimilarityThreshold   float64       `json:"similarity_threshold" yaml:"similarity_threshold"`
	BoostRecentFiles      bool          `json:"boost_recent_files" yaml:"boost_recent_files"`
	BoostPopularFiles     bool          `json:"boost_popular_files" yaml:"boost_popular_files"`
}

// SearchQuery 搜索查询
type SearchQuery struct {
	Query             string                 `json:"query"`
	Tags              []string               `json:"tags"`
	Categories        []string               `json:"categories"`
	FileTypes         []string               `json:"file_types"`
	SizeRange         *SizeRange             `json:"size_range,omitempty"`
	DateRange         *DateRange             `json:"date_range,omitempty"`
	SortBy            SortBy                 `json:"sort_by"`
	SortOrder         SortOrder              `json:"sort_order"`
	Filters           map[string]interface{} `json:"filters,omitempty"`
	IncludeContent    bool                   `json:"include_content"`
	IncludeSimilar    bool                   `json:"include_similar"`
	Limit             int                    `json:"limit"`
	Offset            int                    `json:"offset"`
}

// SizeRange 大小范围
type SizeRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

// DateRange 日期范围
type DateRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// SortBy 排序字段
type SortBy string

const (
	SortByRelevance SortBy = "relevance"
	SortByDate      SortBy = "date"
	SortByName      SortBy = "name"
	SortBySize      SortBy = "size"
	SortByDownloads SortBy = "downloads"
)

// SortOrder 排序顺序
type SortOrder string

const (
	SortOrderAsc  SortOrder = "asc"
	SortOrderDesc SortOrder = "desc"
)

// SearchResult 搜索结果
type SearchResult struct {
	Files       []*SearchResultFile `json:"files"`
	Total       int                 `json:"total"`
	QueryTime   time.Duration       `json:"query_time"`
	Suggestions []string            `json:"suggestions,omitempty"`
	Facets      *SearchFacets       `json:"facets,omitempty"`
	Pagination  *Pagination         `json:"pagination,omitempty"`
}

// SearchResultFile 搜索结果文件
type SearchResultFile struct {
	*types.FileMetadata
	Score      float64   `json:"score"`
	Highlights []string  `json:"highlights,omitempty"`
	Similar    []*SimilarFile `json:"similar,omitempty"`
}

// SimilarFile 相似文件
type SimilarFile struct {
	SHA1     string  `json:"sha1"`
	Filename string  `json:"filename"`
	Score    float64 `json:"score"`
	Reason   string  `json:"reason"`
}

// SearchFacets 搜索分面
type SearchFacets struct {
	Categories map[string]int `json:"categories"`
	FileTypes  map[string]int `json:"file_types"`
	Tags       map[string]int `json:"tags"`
	Sizes      map[string]int `json:"sizes"`
	Dates      map[string]int `json:"dates"`
}

// Pagination 分页信息
type Pagination struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// InvertedIndex 倒排索引
type InvertedIndex struct {
	terms map[string]*TermInfo
	mu    sync.RWMutex
}

// TermInfo 词汇信息
type TermInfo struct {
	Postings map[string]*PostingInfo // SHA1 -> PostingInfo
	DF       int                     // Document Frequency
}

// PostingInfo 倒排列表项
type PostingInfo struct {
	SHA1       string
	Positions  []int
	Frequency  int
	Boost      float64
	LastAccess time.Time
}

// QueryCache 查询缓存
type QueryCache struct {
	entries map[string]*CacheEntry
	mu      sync.RWMutex
	maxSize int
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Result    *SearchResult
	ExpiresAt time.Time
	HitCount  int
}

// NewSearchEngine 创建搜索引擎
func NewSearchEngine(aiService ai.AIService, metadataRepo *repository.MetadataRepository, config *SearchConfig) *SearchEngine {
	if config == nil {
		config = &SearchConfig{
			EnableFullTextSearch: true,
			EnableSemanticSearch: false,
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
	}

	engine := &SearchEngine{
		aiService:    aiService,
		metadataRepo: metadataRepo,
		index:        NewInvertedIndex(),
		config:       config,
		logger:       log.New(log.Writer(), "[SEARCH] ", log.LstdFlags),
		queryCache:   NewQueryCache(1000),
	}

	// 构建索引
	go engine.buildIndex()

	return engine
}

// Search 执行搜索
func (e *SearchEngine) Search(ctx context.Context, query *SearchQuery) (*SearchResult, error) {
	startTime := time.Now()

	// 设置超时
	if e.config.QueryTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.config.QueryTimeout)
		defer cancel()
	}

	// 验证查询
	if err := e.validateQuery(query); err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	// 检查缓存
	cacheKey := e.getCacheKey(query)
	if cached := e.queryCache.Get(cacheKey); cached != nil {
		e.logger.Printf("Cache hit for query: %s", query.Query)
		cached.QueryTime = time.Since(startTime)
		return cached, nil
	}

	// 执行搜索
	result, err := e.executeSearch(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// 后处理
	result = e.postProcess(result, query)

	// 缓存结果
	e.queryCache.Set(cacheKey, result, e.config.CacheExpiration)

	result.QueryTime = time.Since(startTime)
	e.logger.Printf("Search completed in %v: %d results for query '%s'", result.QueryTime, result.Total, query.Query)

	return result, nil
}

// executeSearch 执行实际搜索
func (e *SearchEngine) executeSearch(ctx context.Context, query *SearchQuery) (*SearchResult, error) {
	var results []*SearchResultFile
	var total int

	// 基于元数据的基础搜索
	baseResults, err := e.searchMetadata(ctx, query)
	if err != nil {
		return nil, err
	}

	// 全文搜索
	if e.config.EnableFullTextSearch && query.IncludeContent {
		textResults, err := e.searchFullText(ctx, query)
		if err == nil {
			baseResults = e.mergeResults(baseResults, textResults)
		}
	}

	// 语义搜索
	if e.config.EnableSemanticSearch {
		semanticResults, err := e.searchSemantic(ctx, query)
		if err == nil {
			baseResults = e.mergeResults(baseResults, semanticResults)
		}
	}

	// 模糊搜索
	if e.config.EnableFuzzySearch && len(query.Query) >= 3 {
		fuzzyResults, err := e.searchFuzzy(ctx, query)
		if err == nil {
			baseResults = e.mergeResults(baseResults, fuzzyResults)
		}
	}

	// 过滤和排序
	results = e.filterResults(baseResults, query)
	results = e.sortResults(results, query)

	// 限制结果数量
	if query.Limit > 0 {
		if query.Offset >= len(results) {
			results = []*SearchResultFile{}
		} else {
			end := query.Offset + query.Limit
			if end > len(results) {
				end = len(results)
			}
			results = results[query.Offset:end]
		}
	}

	total = len(baseResults)

	// 生成分面信息
	facets := e.generateFacets(baseResults)

	// 生成建议
	suggestions := []string{}
	if e.config.EnableAutoComplete {
		suggestions = e.generateSuggestions(query.Query)
	}

	// 生成分页信息
	pagination := e.generatePagination(total, query.Limit, query.Offset)

	// 查找相似文件
	if query.IncludeSimilar {
		e.addSimilarFiles(ctx, results, query)
	}

	return &SearchResult{
		Files:       results,
		Total:       total,
		Suggestions: suggestions,
		Facets:      facets,
		Pagination:  pagination,
	}, nil
}

// searchMetadata 基于元数据搜索
func (e *SearchEngine) searchMetadata(ctx context.Context, query *SearchQuery) ([]*SearchResultFile, error) {
	if e.metadataRepo == nil {
		return []*SearchResultFile{}, nil
	}

	files, err := e.metadataRepo.ListFiles(&types.MetadataFilter{})
	if err != nil {
		return nil, err
	}

	var results []*SearchResultFile
	for _, file := range files {
		if e.matchesQuery(file, query) {
			score := e.calculateScore(file, query)
			results = append(results, &SearchResultFile{
				FileMetadata: file,
				Score:        score,
			})
		}
	}

	return results, nil
}

// matchesQuery 检查文件是否匹配查询
func (e *SearchEngine) matchesQuery(file *types.FileMetadata, query *SearchQuery) bool {
	// 文本匹配
	if query.Query != "" {
		text := strings.ToLower(query.Query)
		if !strings.Contains(strings.ToLower(file.FileName), text) &&
			!e.containsAny(strings.ToLower(file.Description), text) &&
			!e.containsAnyText(file.Tags, text) {
			return false
		}
	}

	// 标签匹配
	if len(query.Tags) > 0 && !e.containsAllTags(file.Tags, query.Tags) {
		return false
	}

	// 分类匹配
	if len(query.Categories) > 0 && !e.containsCategory(file, query.Categories) {
		return false
	}

	// 文件类型匹配
	if len(query.FileTypes) > 0 && !e.containsFileType(file, query.FileTypes) {
		return false
	}

	// 大小范围匹配
	if query.SizeRange != nil {
		if query.SizeRange.Min > 0 && file.Size < query.SizeRange.Min {
			return false
		}
		if query.SizeRange.Max > 0 && file.Size > query.SizeRange.Max {
			return false
		}
	}

	// 日期范围匹配
	if query.DateRange != nil {
		if !query.DateRange.From.IsZero() && file.UploadedAt.Before(query.DateRange.From) {
			return false
		}
		if !query.DateRange.To.IsZero() && file.UploadedAt.After(query.DateRange.To) {
			return false
		}
	}

	return true
}

// searchFullText 全文搜索
func (e *SearchEngine) searchFullText(ctx context.Context, query *SearchQuery) ([]*SearchResultFile, error) {
	// 使用倒排索引进行全文搜索
	terms := e.tokenize(query.Query)
	if len(terms) == 0 {
		return []*SearchResultFile{}, nil
	}

	// 获取词汇的倒排列表
	var postings []*PostingInfo
	for _, term := range terms {
		termInfo := e.index.GetTerm(term)
		if termInfo != nil {
			for _, posting := range termInfo.Postings {
				postings = append(postings, posting)
			}
		}
	}

	// 转换为搜索结果
	var results []*SearchResultFile
	for _, posting := range postings {
		file, err := e.metadataRepo.GetMetadata(posting.SHA1)
		if err != nil {
			continue
		}

		score := e.calculateTextScore(posting, terms)
		highlights := e.generateHighlights(file, terms)

		results = append(results, &SearchResultFile{
			FileMetadata: file,
			Score:        score,
			Highlights:   highlights,
		})
	}

	return results, nil
}

// searchSemantic 语义搜索
func (e *SearchEngine) searchSemantic(ctx context.Context, query *SearchQuery) ([]*SearchResultFile, error) {
	// 基于AI分类的语义搜索
	// 这里可以实现更复杂的语义分析，如词向量、BERT等
	return []*SearchResultFile{}, nil
}

// searchFuzzy 模糊搜索
func (e *SearchEngine) searchFuzzy(ctx context.Context, query *SearchQuery) ([]*SearchResultFile, error) {
	// 实现基于编辑距离的模糊搜索
	terms := e.tokenize(query.Query)
	var results []*SearchResultFile

	// 获取所有可能的模糊匹配词汇
	for _, term := range terms {
		similarTerms := e.index.FindSimilarTerms(term, e.config.SimilarityThreshold)
		for _, similarTerm := range similarTerms {
			termInfo := e.index.GetTerm(similarTerm)
			if termInfo != nil {
				for _, posting := range termInfo.Postings {
					file, err := e.metadataRepo.GetMetadata(posting.SHA1)
					if err != nil {
						continue
					}

					score := e.calculateFuzzyScore(posting, term, similarTerm)
					results = append(results, &SearchResultFile{
						FileMetadata: file,
						Score:        score,
					})
				}
			}
		}
	}

	return results, nil
}

// 辅助方法实现...

// validateQuery 验证查询
func (e *SearchEngine) validateQuery(query *SearchQuery) error {
	if len(query.Query) < e.config.MinQueryLength && len(query.Tags) == 0 && len(query.Categories) == 0 {
		return fmt.Errorf("query too short or empty")
	}
	if query.Limit < 0 {
		return fmt.Errorf("invalid limit")
	}
	if query.Offset < 0 {
		return fmt.Errorf("invalid offset")
	}
	return nil
}

// calculateScore 计算文件匹配分数
func (e *SearchEngine) calculateScore(file *types.FileMetadata, query *SearchQuery) float64 {
	score := 0.0

	// 文件名匹配
	if query.Query != "" {
		text := strings.ToLower(query.Query)
		fileName := strings.ToLower(file.FileName)
		if strings.Contains(fileName, text) {
			if fileName == text {
				score += 100.0 // 完全匹配
			} else if strings.HasPrefix(fileName, text) {
				score += 80.0 // 前缀匹配
			} else {
				score += 60.0 // 包含匹配
			}
		}
	}

	// 标签匹配
	if len(query.Tags) > 0 {
		matchedTags := 0
		for _, tag := range query.Tags {
			for _, fileTag := range file.Tags {
				if strings.EqualFold(fileTag, tag) {
					matchedTags++
					score += 20.0
					break
				}
			}
		}
		if matchedTags == len(query.Tags) {
			score += 50.0 // 所有标签都匹配
		}
	}

	// 时间加成
	if e.config.BoostRecentFiles {
		daysSinceUpload := time.Since(file.UploadedAt).Hours() / 24
		if daysSinceUpload < 1 {
			score += 10.0
		} else if daysSinceUpload < 7 {
			score += 5.0
		}
	}

	return score
}

// 更多辅助方法...

// buildIndex 构建索引
func (e *SearchEngine) buildIndex() {
	e.logger.Println("Building search index...")

	if e.metadataRepo == nil {
		return
	}

	files, err := e.metadataRepo.ListFiles(&types.MetadataFilter{})
	if err != nil {
		e.logger.Printf("Failed to get files for indexing: %v", err)
		return
	}

	for _, file := range files {
		e.indexFile(file)
	}

	e.logger.Printf("Search index built with %d files", len(files))
}

// indexFile 为单个文件建立索引
func (e *SearchEngine) indexFile(file *types.FileMetadata) {
	// 为文件名建立索引
	terms := e.tokenize(file.FileName)
	for _, term := range terms {
		e.index.AddPosting(term, file.SHA1, 1.0)
	}

	// 为描述建立索引
	if file.Description != "" {
		terms = e.tokenize(file.Description)
		for _, term := range terms {
			e.index.AddPosting(term, file.SHA1, 0.8)
		}
	}

	// 为标签建立索引
	for _, tag := range file.Tags {
		terms = e.tokenize(tag)
		for _, term := range terms {
			e.index.AddPosting(term, file.SHA1, 0.9)
		}
	}
}

// tokenize 分词
func (e *SearchEngine) tokenize(text string) []string {
	text = strings.ToLower(text)
	terms := strings.Fields(text)

	// 简单的过滤和标准化
	var filtered []string
	for _, term := range terms {
		term = strings.Trim(term, ".,!?;:()[]{}\"'")
		if len(term) >= 2 {
			filtered = append(filtered, term)
		}
	}

	return filtered
}

// 后处理和其他方法的占位符实现...
func (e *SearchEngine) postProcess(result *SearchResult, query *SearchQuery) *SearchResult {
	return result
}

func (e *SearchEngine) filterResults(results []*SearchResultFile, query *SearchQuery) []*SearchResultFile {
	return results
}

func (e *SearchEngine) sortResults(results []*SearchResultFile, query *SearchQuery) []*SearchResultFile {
	sort.Slice(results, func(i, j int) bool {
		if query.SortOrder == SortOrderDesc {
			return results[i].Score > results[j].Score
		}
		return results[i].Score < results[j].Score
	})
	return results
}

func (e *SearchEngine) mergeResults(a, b []*SearchResultFile) []*SearchResultFile {
	seen := make(map[string]bool)
	var merged []*SearchResultFile

	for _, result := range a {
		if !seen[result.SHA1] {
			merged = append(merged, result)
			seen[result.SHA1] = true
		}
	}

	for _, result := range b {
		if !seen[result.SHA1] {
			merged = append(merged, result)
			seen[result.SHA1] = true
		}
	}

	return merged
}

func (e *SearchEngine) generateFacets(results []*SearchResultFile) *SearchFacets {
	return &SearchFacets{
		Categories: make(map[string]int),
		FileTypes:  make(map[string]int),
		Tags:       make(map[string]int),
		Sizes:      make(map[string]int),
		Dates:      make(map[string]int),
	}
}

func (e *SearchEngine) generateSuggestions(query string) []string {
	// 简单的建议生成
	return []string{}
}

func (e *SearchEngine) generatePagination(total, limit, offset int) *Pagination {
	if limit <= 0 {
		limit = 20
	}

	page := (offset / limit) + 1
	totalPages := (total + limit - 1) / limit

	return &Pagination{
		Page:       page,
		PerPage:    limit,
		Total:      total,
		TotalPages: totalPages,
	}
}

func (e *SearchEngine) addSimilarFiles(ctx context.Context, results []*SearchResultFile, query *SearchQuery) {
	// 为每个结果查找相似文件
	for i, result := range results {
		if i >= 5 { // 只为前5个结果查找相似文件
			break
		}

		similar, err := e.aiService.GetSimilarFiles(ctx, result.SHA1, 3)
		if err != nil {
			continue
		}

		for _, simFile := range similar {
			result.Similar = append(result.Similar, &SimilarFile{
				SHA1:     simFile.SHA1,
				Filename: simFile.FileName,
				Score:    0.8, // 占位符分数
				Reason:   "Similar content",
			})
		}
	}
}

func (e *SearchEngine) getCacheKey(query *SearchQuery) string {
	// 生成缓存键
	return fmt.Sprintf("%s_%v_%v_%v", query.Query, query.Tags, query.Categories, query.FileTypes)
}

// 辅助函数
func (e *SearchEngine) containsAny(text, substr string) bool {
	return strings.Contains(text, substr)
}

func (e *SearchEngine) containsAnyText(tags []string, text string) bool {
	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag), text) {
			return true
		}
	}
	return false
}

func (e *SearchEngine) containsAllTags(fileTags, queryTags []string) bool {
	for _, qTag := range queryTags {
		found := false
		for _, fTag := range fileTags {
			if strings.EqualFold(fTag, qTag) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (e *SearchEngine) containsCategory(file *types.FileMetadata, categories []string) bool {
	// 从AI分类中获取类别
	if file.CustomFields != nil {
		if aiClass, exists := file.CustomFields["ai_classification"]; exists {
			// 这里应该解析JSON并检查类别，简化处理
			for _, cat := range categories {
				if strings.Contains(aiClass, cat) {
					return true
				}
			}
		}
	}
	return false
}

func (e *SearchEngine) containsFileType(file *types.FileMetadata, fileTypes []string) bool {
	for _, ft := range fileTypes {
		if strings.HasSuffix(strings.ToLower(file.FileName), strings.ToLower(ft)) {
			return true
		}
	}
	return false
}

func (e *SearchEngine) calculateTextScore(posting *PostingInfo, terms []string) float64 {
	// 基于词频和位置计算文本分数
	score := float64(posting.Frequency)

	// 时间衰减
	daysSinceAccess := time.Since(posting.LastAccess).Hours() / 24
	score *= (1.0 / (1.0 + daysSinceAccess*0.1))

	return score
}

func (e *SearchEngine) generateHighlights(file *types.FileMetadata, terms []string) []string {
	var highlights []string

	for _, term := range terms {
		if strings.Contains(strings.ToLower(file.FileName), term) {
			highlights = append(highlights, fmt.Sprintf("文件名包含: %s", term))
		}
		if file.Description != "" && strings.Contains(strings.ToLower(file.Description), term) {
			highlights = append(highlights, fmt.Sprintf("描述包含: %s", term))
		}
	}

	return highlights
}

func (e *SearchEngine) calculateFuzzyScore(posting *PostingInfo, originalTerm, matchedTerm string) float64 {
	// 基于编辑距离计算模糊匹配分数
	score := float64(posting.Frequency) * 0.7 // 模糊匹配分数降低

	// 如果词汇长度相近，给予额外加成
	diff := len(originalTerm) - len(matchedTerm)
	if diff < 0 {
		diff = -diff
	}
	if diff <= 1 {
		score *= 1.2
	}

	return score
}

// 创建倒排索引
func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{
		terms: make(map[string]*TermInfo),
	}
}

func (idx *InvertedIndex) AddPosting(term, sha1 string, boost float64) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.terms[term] == nil {
		idx.terms[term] = &TermInfo{
			Postings: make(map[string]*PostingInfo),
		}
	}

	termInfo := idx.terms[term]
	if termInfo.Postings[sha1] == nil {
		termInfo.Postings[sha1] = &PostingInfo{
			SHA1:       sha1,
			Boost:      boost,
			LastAccess: time.Now(),
		}
		termInfo.DF++
	} else {
		posting := termInfo.Postings[sha1]
		posting.Frequency++
		posting.LastAccess = time.Now()
	}
}

func (idx *InvertedIndex) GetTerm(term string) *TermInfo {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.terms[term]
}

func (idx *InvertedIndex) FindSimilarTerms(term string, threshold float64) []string {
	// 简单的相似词汇查找
	var similar []string

	for t := range idx.terms {
		if similarity := idx.calculateSimilarity(term, t); similarity >= threshold {
			similar = append(similar, t)
		}
	}

	return similar
}

func (idx *InvertedIndex) calculateSimilarity(a, b string) float64 {
	// 简单的相似度计算
	if a == b {
		return 1.0
	}

	// 这里可以实现更复杂的相似度算法，如编辑距离、Jaccard相似度等
	// 目前使用简单的包含关系
	if len(a) > 0 && len(b) > 0 {
		// 检查是否有共同字符
		aSet := make(map[rune]bool)
		bSet := make(map[rune]bool)

		for _, r := range a {
			aSet[r] = true
		}
		for _, r := range b {
			bSet[r] = true
		}

		intersection := 0
		for r := range aSet {
			if bSet[r] {
				intersection++
			}
		}

		union := len(aSet) + len(bSet) - intersection
		if union > 0 {
			return float64(intersection) / float64(union)
		}
	}

	return 0.0
}

func (e *SearchEngine) calculateSimilarity(a, b string) float64 {
	// 简单的相似度计算
	if a == b {
		return 1.0
	}

	// 这里可以实现更复杂的相似度算法，如编辑距离、Jaccard相似度等
	// 目前使用简单的包含关系
	if strings.Contains(b, a) || strings.Contains(a, b) {
		return 0.8
	}

	return 0.0
}

// 创建查询缓存
func NewQueryCache(maxSize int) *QueryCache {
	return &QueryCache{
		entries: make(map[string]*CacheEntry),
		maxSize: maxSize,
	}
}

func (cache *QueryCache) Get(key string) *SearchResult {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	entry := cache.entries[key]
	if entry == nil || time.Now().After(entry.ExpiresAt) {
		return nil
	}

	entry.HitCount++
	return entry.Result
}

func (cache *QueryCache) Set(key string, result *SearchResult, expiration time.Duration) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	// 简单的LRU：如果缓存满了，删除最旧的条目
	if len(cache.entries) >= cache.maxSize {
		var oldestKey string
		var oldestTime time.Time

		for k, entry := range cache.entries {
			if oldestKey == "" || entry.ExpiresAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = entry.ExpiresAt
			}
		}

		if oldestKey != "" {
			delete(cache.entries, oldestKey)
		}
	}

	cache.entries[key] = &CacheEntry{
		Result:    result,
		ExpiresAt: time.Now().Add(expiration),
		HitCount:  0,
	}
}