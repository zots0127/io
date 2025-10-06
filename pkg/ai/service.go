package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"sync"
	"time"

	"github.com/zots0127/io/pkg/metadata/repository"
	"github.com/zots0127/io/pkg/types"
)

// AIService AI服务接口
type AIService interface {
	ClassifyFile(ctx context.Context, sha1 string, file *multipart.FileHeader) (*ClassificationResult, error)
	AnalyzeFile(ctx context.Context, sha1 string, file *multipart.FileHeader) (*AnalysisResult, error)
	BatchClassify(ctx context.Context, files map[string]*multipart.FileHeader) (map[string]*ClassificationResult, error)
	BatchAnalyze(ctx context.Context, files map[string]*multipart.FileHeader) (map[string]*AnalysisResult, error)
	UpdateClassification(ctx context.Context, sha1 string, result *ClassificationResult) error
	GetSimilarFiles(ctx context.Context, sha1 string, limit int) ([]*types.FileMetadata, error)
	SearchByTags(ctx context.Context, tags []string, limit int) ([]*types.FileMetadata, error)
	GetInsights(ctx context.Context, timeRange string) (*InsightsResult, error)
	GetConfig() *AIServiceConfig
	UpdateConfig(config *AIServiceConfig)
	GetStats() map[string]interface{}
	Health() error
}

// AIServiceImpl AI服务实现
type AIServiceImpl struct {
	classifier     *Classifier
	analyzer       *ContentAnalyzer
	metadataRepo   *repository.MetadataRepository
	config         *AIServiceConfig
	logger         *log.Logger
	processingJobs sync.Map // 正在处理的任务
}

// AIServiceConfig AI服务配置
type AIServiceConfig struct {
	EnableClassification bool          `json:"enable_classification" yaml:"enable_classification"`
	EnableAnalysis       bool          `json:"enable_analysis" yaml:"enable_analysis"`
	EnableBatchProcess   bool          `json:"enable_batch_process" yaml:"enable_batch_process"`
	MaxBatchSize         int           `json:"max_batch_size" yaml:"max_batch_size"`
	ProcessingTimeout    time.Duration `json:"processing_timeout" yaml:"processing_timeout"`
	EnableSimilarity     bool          `json:"enable_similarity" yaml:"enable_similarity"`
	SimilarityThreshold  float64       `json:"similarity_threshold" yaml:"similarity_threshold"`
	EnableAutoTagging    bool          `json:"enable_auto_tagging" yaml:"enable_auto_tagging"`
	EnableInsights       bool          `json:"enable_insights" yaml:"enable_insights"`
	InsightsCacheTime    time.Duration `json:"insights_cache_time" yaml:"insights_cache_time"`
}

// InsightsResult 洞察结果
type InsightsResult struct {
	TimeRange    string                 `json:"time_range"`
	TotalFiles   int                    `json:"total_files"`
	Categories   map[string]int         `json:"categories"`
	Tags         map[string]int         `json:"tags"`
	Trends       map[string]interface{} `json:"trends"`
	StorageUsage StorageUsageStats      `json:"storage_usage"`
	Activity     ActivityStats          `json:"activity"`
	GeneratedAt  time.Time              `json:"generated_at"`
	CacheUntil   time.Time              `json:"cache_until"`
}

// StorageUsageStats 存储使用统计
type StorageUsageStats struct {
	TotalSize     int64            `json:"total_size"`
	ByCategory    map[string]int64 `json:"by_category"`
	ByFileType    map[string]int64 `json:"by_file_type"`
	LargestFiles  []FileInfo       `json:"largest_files"`
	AverageSize   int64            `json:"average_size"`
	GrowthRate    float64          `json:"growth_rate"`
}

// ActivityStats 活动统计
type ActivityStats struct {
	UploadsPerDay     map[string]int `json:"uploads_per_day"`
	DownloadsPerDay   map[string]int `json:"downloads_per_day"`
	PeakHours         []int          `json:"peak_hours"`
	MostActiveTags    []string       `json:"most_active_tags"`
	RecentActivity    []ActivityItem `json:"recent_activity"`
}

// ActivityItem 活动项
type ActivityItem struct {
	Type      string    `json:"type"`
	SHA1      string    `json:"sha1"`
	Filename  string    `json:"filename"`
	Timestamp time.Time `json:"timestamp"`
	User      string    `json:"user,omitempty"`
}


// NewAIService 创建AI服务
func NewAIService(metadataRepo *repository.MetadataRepository, config *AIServiceConfig) AIService {
	if config == nil {
		config = &AIServiceConfig{
			EnableClassification: true,
			EnableAnalysis:       true,
			EnableBatchProcess:   true,
			MaxBatchSize:         100,
			ProcessingTimeout:    30 * time.Second,
			EnableSimilarity:     false,
			SimilarityThreshold:  0.8,
			EnableAutoTagging:    true,
			EnableInsights:       true,
			InsightsCacheTime:    5 * time.Minute,
		}
	}

	classifierConfig := &ClassifierConfig{
		EnableCache:           true,
		CacheExpiration:       24 * time.Hour,
		MaxCacheSize:          10000,
		EnableParallel:        true,
		MaxParallelWorkers:    4,
		DefaultConfidence:     0.8,
		EnableOCR:             true,
		EnableObjectDetection: false,
		ModelsPath:            "./models",
	}

	analyzerConfig := &AnalyzerConfig{
		EnableDeepAnalysis:    true,
		MaxAnalysisSize:       10 * 1024 * 1024,
		AnalysisTimeout:       30 * time.Second,
		EnableSimilarityCheck: config.EnableSimilarity,
		SimilarityThreshold:   config.SimilarityThreshold,
		EnableContentSummary:  true,
		SummaryMaxLength:      500,
	}

	classifier := NewClassifier(classifierConfig)
	analyzer := NewContentAnalyzer(classifier, analyzerConfig)

	return &AIServiceImpl{
		classifier:   classifier,
		analyzer:     analyzer,
		metadataRepo: metadataRepo,
		config:       config,
		logger:       log.New(log.Writer(), "[AI] ", log.LstdFlags),
	}
}

// ClassifyFile 分类文件
func (s *AIServiceImpl) ClassifyFile(ctx context.Context, sha1 string, file *multipart.FileHeader) (*ClassificationResult, error) {
	if !s.config.EnableClassification {
		return nil, fmt.Errorf("classification is disabled")
	}

	// 检查是否已经在处理中
	if _, processing := s.processingJobs.Load(sha1); processing {
		return nil, fmt.Errorf("file is already being processed")
	}

	// 标记为正在处理
	s.processingJobs.Store(sha1, true)
	defer s.processingJobs.Delete(sha1)

	// 设置超时
	if s.config.ProcessingTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.config.ProcessingTimeout)
		defer cancel()
	}

	// 执行分类
	result, err := s.classifier.ClassifyFile(ctx, sha1, file)
	if err != nil {
		s.logger.Printf("Classification failed for %s: %v", sha1, err)
		return nil, err
	}

	// 更新元数据
	if s.config.EnableAutoTagging {
		err = s.UpdateClassification(ctx, sha1, result)
		if err != nil {
			s.logger.Printf("Failed to update classification for %s: %v", sha1, err)
		}
	}

	s.logger.Printf("Successfully classified file %s: %s/%s", sha1, result.Category, result.Subcategory)
	return result, nil
}

// AnalyzeFile 分析文件
func (s *AIServiceImpl) AnalyzeFile(ctx context.Context, sha1 string, file *multipart.FileHeader) (*AnalysisResult, error) {
	if !s.config.EnableAnalysis {
		return nil, fmt.Errorf("analysis is disabled")
	}

	// 检查是否已经在处理中
	if _, processing := s.processingJobs.Load(sha1); processing {
		return nil, fmt.Errorf("file is already being processed")
	}

	// 标记为正在处理
	s.processingJobs.Store(sha1, true)
	defer s.processingJobs.Delete(sha1)

	// 设置超时
	if s.config.ProcessingTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.config.ProcessingTimeout)
		defer cancel()
	}

	// 执行分析
	result, err := s.analyzer.AnalyzeFile(ctx, sha1, file)
	if err != nil {
		s.logger.Printf("Analysis failed for %s: %v", sha1, err)
		return nil, err
	}

	s.logger.Printf("Successfully analyzed file %s in %v", sha1, result.ProcessingTime)
	return result, nil
}

// BatchClassify 批量分类
func (s *AIServiceImpl) BatchClassify(ctx context.Context, files map[string]*multipart.FileHeader) (map[string]*ClassificationResult, error) {
	if !s.config.EnableBatchProcess {
		return nil, fmt.Errorf("batch processing is disabled")
	}

	if len(files) > s.config.MaxBatchSize {
		return nil, fmt.Errorf("batch size exceeds maximum allowed: %d", s.config.MaxBatchSize)
	}

	s.logger.Printf("Starting batch classification for %d files", len(files))

	results := make(map[string]*ClassificationResult)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 限制并发数
	semaphore := make(chan struct{}, 10)

	for sha1, file := range files {
		wg.Add(1)
		go func(sha1 string, file *multipart.FileHeader) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 处理单个文件
			result, err := s.ClassifyFile(ctx, sha1, file)
			if err != nil {
				s.logger.Printf("Failed to classify file %s: %v", sha1, err)
				return
			}

			mu.Lock()
			results[sha1] = result
			mu.Unlock()

		}(sha1, file)
	}

	wg.Wait()

	s.logger.Printf("Batch classification completed: %d/%d files processed", len(results), len(files))
	return results, nil
}

// BatchAnalyze 批量分析
func (s *AIServiceImpl) BatchAnalyze(ctx context.Context, files map[string]*multipart.FileHeader) (map[string]*AnalysisResult, error) {
	if !s.config.EnableBatchProcess {
		return nil, fmt.Errorf("batch processing is disabled")
	}

	if len(files) > s.config.MaxBatchSize {
		return nil, fmt.Errorf("batch size exceeds maximum allowed: %d", s.config.MaxBatchSize)
	}

	s.logger.Printf("Starting batch analysis for %d files", len(files))

	results := make(map[string]*AnalysisResult)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 限制并发数
	semaphore := make(chan struct{}, 5) // 分析更耗时，限制更少并发

	for sha1, file := range files {
		wg.Add(1)
		go func(sha1 string, file *multipart.FileHeader) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 处理单个文件
			result, err := s.AnalyzeFile(ctx, sha1, file)
			if err != nil {
				s.logger.Printf("Failed to analyze file %s: %v", sha1, err)
				return
			}

			mu.Lock()
			results[sha1] = result
			mu.Unlock()

		}(sha1, file)
	}

	wg.Wait()

	s.logger.Printf("Batch analysis completed: %d/%d files processed", len(results), len(files))
	return results, nil
}

// UpdateClassification 更新分类信息到元数据
func (s *AIServiceImpl) UpdateClassification(ctx context.Context, sha1 string, result *ClassificationResult) error {
	if s.metadataRepo == nil {
		return fmt.Errorf("metadata repository not available")
	}

	// 获取现有元数据
	metadata, err := s.metadataRepo.GetMetadata(sha1)
	if err != nil {
		return fmt.Errorf("failed to get metadata: %w", err)
	}

	// 更新AI分类信息
	if metadata.CustomFields == nil {
		metadata.CustomFields = make(map[string]string)
	}

	// 将AI分类信息存储为JSON字符串
	aiClass := map[string]interface{}{
		"content_type":   result.ContentType,
		"category":       result.Category,
		"subcategory":    result.Subcategory,
		"confidence":     result.Confidence,
		"tags":           result.Tags,
		"model_version":  result.ModelVersion,
		"processed_at":   result.ProcessedAt,
	}

	aiClassJSON, _ := json.Marshal(aiClass)
	metadata.CustomFields["ai_classification"] = string(aiClassJSON)

	// 合并标签
	mergedTags := MergeTags(metadata.Tags, result.Tags)
	metadata.Tags = mergedTags

	// 保存更新后的元数据
	err = s.metadataRepo.SaveMetadata(metadata)
	if err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}

// GetSimilarFiles 获取相似文件
func (s *AIServiceImpl) GetSimilarFiles(ctx context.Context, sha1 string, limit int) ([]*types.FileMetadata, error) {
	if !s.config.EnableSimilarity {
		return nil, fmt.Errorf("similarity feature is disabled")
	}

	// 获取源文件的分类信息
	metadata, err := s.metadataRepo.GetMetadata(sha1)
	if err != nil {
		return nil, fmt.Errorf("failed to get source metadata: %w", err)
	}

	var similarFiles []*types.FileMetadata

	// 简单的相似性检测：基于标签
	for _, tag := range metadata.Tags {
		files, err := s.SearchByTags(ctx, []string{tag}, limit)
		if err != nil {
			continue
		}
		for _, file := range files {
			if file.SHA1 != sha1 { // 排除自己
				similarFiles = append(similarFiles, file)
				if len(similarFiles) >= limit {
					return similarFiles, nil
				}
			}
		}
	}

	return similarFiles, nil
}

// SearchByTags 根据标签搜索文件
func (s *AIServiceImpl) SearchByTags(ctx context.Context, tags []string, limit int) ([]*types.FileMetadata, error) {
	if s.metadataRepo == nil {
		return nil, fmt.Errorf("metadata repository not available")
	}

	// 这里应该实现真正的标签搜索逻辑
	// 目前返回模拟结果
	return []*types.FileMetadata{}, nil
}

// GetInsights 获取数据洞察
func (s *AIServiceImpl) GetInsights(ctx context.Context, timeRange string) (*InsightsResult, error) {
	if !s.config.EnableInsights {
		return nil, fmt.Errorf("insights feature is disabled")
	}

	// 这里应该实现真正的洞察分析逻辑
	// 目前返回模拟结果
	insights := &InsightsResult{
		TimeRange:   timeRange,
		TotalFiles:  1000,
		Categories:  map[string]int{"文档": 400, "图片": 300, "视频": 200, "其他": 100},
		Tags:        map[string]int{"重要": 200, "工作": 150, "个人": 100},
		GeneratedAt: time.Now(),
		CacheUntil:  time.Now().Add(s.config.InsightsCacheTime),
	}

	return insights, nil
}

// GetConfig 获取服务配置
func (s *AIServiceImpl) GetConfig() *AIServiceConfig {
	return s.config
}

// UpdateConfig 更新服务配置
func (s *AIServiceImpl) UpdateConfig(config *AIServiceConfig) {
	s.config = config
}

// GetStats 获取服务统计
func (s *AIServiceImpl) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"enabled_classification": s.config.EnableClassification,
		"enabled_analysis":       s.config.EnableAnalysis,
		"enabled_batch_process":  s.config.EnableBatchProcess,
		"max_batch_size":         s.config.MaxBatchSize,
		"processing_jobs":        0,
		"cache_stats":            s.classifier.GetCacheStats(),
	}

	// 计算正在处理的任务数
	s.processingJobs.Range(func(key, value interface{}) bool {
		stats["processing_jobs"] = stats["processing_jobs"].(int) + 1
		return true
	})

	return stats
}

// Health 健康检查
func (s *AIServiceImpl) Health() error {
	if s.classifier == nil {
		return fmt.Errorf("classifier not initialized")
	}
	if s.analyzer == nil {
		return fmt.Errorf("analyzer not initialized")
	}
	return nil
}