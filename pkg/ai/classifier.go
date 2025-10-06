package ai

import (
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ContentType 文件内容类型枚举
type ContentType string

const (
	ContentTypeDocument ContentType = "document"
	ContentTypeImage    ContentType = "image"
	ContentTypeVideo    ContentType = "video"
	ContentTypeAudio    ContentType = "audio"
	ContentTypeArchive  ContentType = "archive"
	ContentTypeCode     ContentType = "code"
	ContentTypeData     ContentType = "data"
	ContentTypeOther    ContentType = "other"
)

// ClassificationResult 分类结果
type ClassificationResult struct {
	ContentType   ContentType           `json:"content_type"`
	Category      string                `json:"category"`
	Subcategory   string                `json:"subcategory"`
	Confidence    float64               `json:"confidence"`
	Tags          []string              `json:"tags"`
	Metadata      map[string]interface{} `json:"metadata"`
	ProcessedAt   time.Time             `json:"processed_at"`
	ModelVersion  string                `json:"model_version"`
}

// AIModel AI模型接口
type AIModel interface {
	Analyze(ctx context.Context, filePath string, file *multipart.FileHeader) (*ClassificationResult, error)
	ClassifyContent(ctx context.Context, data []byte, mimeType string) (*ClassificationResult, error)
	ExtractText(ctx context.Context, filePath string) (string, error)
	DetectObjects(ctx context.Context, filePath string) ([]string, error)
	GetModelInfo() ModelInfo
}

// ModelInfo 模型信息
type ModelInfo struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Type        string    `json:"type"`
	Accuracy    float64   `json:"accuracy"`
	LastUpdated time.Time `json:"last_updated"`
	Status      string    `json:"status"`
}

// Classifier 文件分类器
type Classifier struct {
	models    map[string]AIModel
	config    *ClassifierConfig
	logger    *log.Logger
	cache     *ClassificationCache
	mu        sync.RWMutex
}

// ClassifierConfig 分类器配置
type ClassifierConfig struct {
	EnableCache           bool          `json:"enable_cache" yaml:"enable_cache"`
	CacheExpiration       time.Duration `json:"cache_expiration" yaml:"cache_expiration"`
	MaxCacheSize          int           `json:"max_cache_size" yaml:"max_cache_size"`
	EnableParallel        bool          `json:"enable_parallel" yaml:"enable_parallel"`
	MaxParallelWorkers    int           `json:"max_parallel_workers" yaml:"max_parallel_workers"`
	DefaultConfidence     float64       `json:"default_confidence" yaml:"default_confidence"`
	EnableOCR             bool          `json:"enable_ocr" yaml:"enable_ocr"`
	EnableObjectDetection bool          `json:"enable_object_detection" yaml:"enable_object_detection"`
	ModelsPath            string        `json:"models_path" yaml:"models_path"`
}

// ClassificationCache 分类缓存
type ClassificationCache struct {
	entries map[string]*CacheEntry
	mu      sync.RWMutex
	maxSize int
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Result    *ClassificationResult
	ExpiresAt time.Time
}

// NewClassifier 创建新的分类器
func NewClassifier(config *ClassifierConfig) *Classifier {
	if config == nil {
		config = &ClassifierConfig{
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
	}

	classifier := &Classifier{
		models: make(map[string]AIModel),
		config: config,
		cache:  NewClassificationCache(config.MaxCacheSize),
	}

	// 初始化默认模型
	classifier.initializeDefaultModels()

	return classifier
}

// initializeDefaultModels 初始化默认模型
func (c *Classifier) initializeDefaultModels() {
	// 内容类型识别模型
	c.models["content_type"] = NewContentTypeModel()

	// 标签生成模型
	c.models["tagger"] = NewTaggerModel(c.config)

	// 文本提取模型
	if c.config.EnableOCR {
		c.models["ocr"] = NewOCRModel(c.config)
	}

	// 对象检测模型
	if c.config.EnableObjectDetection {
		c.models["object_detection"] = NewObjectDetectionModel(c.config)
	}
}

// RegisterModel 注册AI模型
func (c *Classifier) RegisterModel(name string, model AIModel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.models[name] = model
}

// ClassifyFile 分类文件
func (c *Classifier) ClassifyFile(ctx context.Context, filePath string, file *multipart.FileHeader) (*ClassificationResult, error) {
	// 检查缓存
	if c.config.EnableCache {
		if cached := c.cache.Get(filePath, file.Size); cached != nil {
			return cached, nil
		}
	}

	// 创建分类结果
	result := &ClassificationResult{
		ProcessedAt:  time.Now(),
		ModelVersion: "v1.0.0",
		Metadata:     make(map[string]interface{}),
		Tags:         make([]string, 0),
	}

	// 并行或串行处理
	if c.config.EnableParallel {
		result = c.classifyParallel(ctx, filePath, file)
	} else {
		result = c.classifySequential(ctx, filePath, file)
	}

	// 缓存结果
	if c.config.EnableCache && result != nil {
		c.cache.Set(filePath, file.Size, result, c.config.CacheExpiration)
	}

	return result, nil
}

// classifySequential 串行分类
func (c *Classifier) classifySequential(ctx context.Context, filePath string, file *multipart.FileHeader) *ClassificationResult {
	result := &ClassificationResult{
		ProcessedAt:  time.Now(),
		ModelVersion: "v1.0.0",
		Metadata:     make(map[string]interface{}),
		Tags:         make([]string, 0),
	}

	// 内容类型分类
	if contentModel, exists := c.models["content_type"]; exists {
		if contentResult, err := contentModel.Analyze(ctx, filePath, file); err == nil {
			result.ContentType = contentResult.ContentType
			result.Category = contentResult.Category
			result.Subcategory = contentResult.Subcategory
			result.Confidence = contentResult.Confidence
		}
	}

	// 标签生成
	if taggerModel, exists := c.models["tagger"]; exists {
		if tagResult, err := taggerModel.Analyze(ctx, filePath, file); err == nil {
			result.Tags = append(result.Tags, tagResult.Tags...)
			// 合并元数据
			for k, v := range tagResult.Metadata {
				result.Metadata[k] = v
			}
		}
	}

	// OCR文本提取
	if ocrModel, exists := c.models["ocr"]; exists {
		if text, err := ocrModel.ExtractText(ctx, filePath); err == nil {
			result.Metadata["extracted_text"] = text
			result.Metadata["text_length"] = len(text)
		}
	}

	// 对象检测
	if objModel, exists := c.models["object_detection"]; exists {
		if objects, err := objModel.DetectObjects(ctx, filePath); err == nil {
			result.Metadata["detected_objects"] = objects
			result.Tags = append(result.Tags, objects...)
		}
	}

	return result
}

// classifyParallel 并行分类
func (c *Classifier) classifyParallel(ctx context.Context, filePath string, file *multipart.FileHeader) *ClassificationResult {
	result := &ClassificationResult{
		ProcessedAt:  time.Now(),
		ModelVersion: "v1.0.0",
		Metadata:     make(map[string]interface{}),
		Tags:         make([]string, 0),
	}

	var wg sync.WaitGroup
	results := make(chan *ClassificationResult, len(c.models))
	errors := make(chan error, len(c.models))

	// 启动所有模型
	for name, model := range c.models {
		wg.Add(1)
		go func(modelName string, model AIModel) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				errors <- ctx.Err()
				return
			default:
			}

			switch modelName {
			case "content_type":
				if res, err := model.Analyze(ctx, filePath, file); err == nil {
					results <- res
				} else {
					errors <- err
				}
			case "tagger":
				if res, err := model.Analyze(ctx, filePath, file); err == nil {
					results <- res
				} else {
					errors <- err
				}
			case "ocr":
				if text, err := model.ExtractText(ctx, filePath); err == nil {
					res := &ClassificationResult{
						Metadata: map[string]interface{}{
							"extracted_text": text,
							"text_length":    len(text),
						},
					}
					results <- res
				} else {
					errors <- err
				}
			case "object_detection":
				if objects, err := model.DetectObjects(ctx, filePath); err == nil {
					res := &ClassificationResult{
						Metadata: map[string]interface{}{
							"detected_objects": objects,
						},
						Tags: objects,
					}
					results <- res
				} else {
					errors <- err
				}
			}
		}(name, model)
	}

	// 等待所有goroutine完成
	wg.Wait()
	close(results)
	close(errors)

	// 收集结果
	for res := range results {
		if res.ContentType != "" {
			result.ContentType = res.ContentType
			result.Category = res.Category
			result.Subcategory = res.Subcategory
			result.Confidence = res.Confidence
		}

		if len(res.Tags) > 0 {
			result.Tags = append(result.Tags, res.Tags...)
		}

		// 合并元数据
		for k, v := range res.Metadata {
			result.Metadata[k] = v
		}
	}

	// 记录错误（如果有）
	for err := range errors {
		if c.logger != nil {
			c.logger.Printf("Classification error: %v", err)
		}
	}

	return result
}

// ClassifyByContent 根据内容分类
func (c *Classifier) ClassifyByContent(ctx context.Context, data []byte, mimeType string) (*ClassificationResult, error) {
	if contentModel, exists := c.models["content_type"]; exists {
		return contentModel.ClassifyContent(ctx, data, mimeType)
	}
	return nil, fmt.Errorf("content type model not available")
}

// GetSupportedModels 获取支持的模型列表
func (c *Classifier) GetSupportedModels() map[string]ModelInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	models := make(map[string]ModelInfo)
	for name, model := range c.models {
		models[name] = model.GetModelInfo()
	}
	return models
}

// ClearCache 清空缓存
func (c *Classifier) ClearCache() {
	if c.cache != nil {
		c.cache.Clear()
	}
}

// GetCacheStats 获取缓存统计
func (c *Classifier) GetCacheStats() map[string]interface{} {
	if c.cache != nil {
		return c.cache.GetStats()
	}
	return map[string]interface{}{
		"enabled": false,
	}
}

// UpdateConfig 更新配置
func (c *Classifier) UpdateConfig(config *ClassifierConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
}

// GetConfig 获取当前配置
func (c *Classifier) GetConfig() *ClassifierConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// NewClassificationCache 创建分类缓存
func NewClassificationCache(maxSize int) *ClassificationCache {
	return &ClassificationCache{
		entries: make(map[string]*CacheEntry),
		maxSize: maxSize,
	}
}

// Set 设置缓存
func (cache *ClassificationCache) Set(key string, size int64, result *ClassificationResult, expiration time.Duration) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	// 如果缓存已满，删除最旧的条目
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
	}
}

// Get 获取缓存
func (cache *ClassificationCache) Get(key string, size int64) *ClassificationResult {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	if entry, exists := cache.entries[key]; exists {
		if time.Now().Before(entry.ExpiresAt) {
			return entry.Result
		}
		// 过期条目
		delete(cache.entries, key)
	}
	return nil
}

// Clear 清空缓存
func (cache *ClassificationCache) Clear() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.entries = make(map[string]*CacheEntry)
}

// GetStats 获取缓存统计
func (cache *ClassificationCache) GetStats() map[string]interface{} {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	validEntries := 0
	for _, entry := range cache.entries {
		if time.Now().Before(entry.ExpiresAt) {
			validEntries++
		}
	}

	return map[string]interface{}{
		"enabled":      true,
		"total_entries": len(cache.entries),
		"valid_entries": validEntries,
		"max_size":     cache.maxSize,
		"hit_rate":     0.0, // 需要跟踪命中次数
	}
}

// 辅助函数

// DetectContentTypeFromExtension 从扩展名检测内容类型
func DetectContentTypeFromExtension(filename string) ContentType {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".txt", ".md", ".doc", ".docx", ".pdf", ".rtf":
		return ContentTypeDocument
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".svg":
		return ContentTypeImage
	case ".mp4", ".avi", ".mov", ".wmv", ".flv", ".mkv":
		return ContentTypeVideo
	case ".mp3", ".wav", ".flac", ".aac", ".ogg":
		return ContentTypeAudio
	case ".zip", ".rar", ".7z", ".tar", ".gz":
		return ContentTypeArchive
	case ".go", ".js", ".py", ".java", ".cpp", ".c", ".h":
		return ContentTypeCode
	case ".json", ".xml", ".csv", ".yaml", ".yml":
		return ContentTypeData
	default:
		return ContentTypeOther
	}
}

// GetCategoryFromContentType 从内容类型获取分类
func GetCategoryFromContentType(contentType ContentType) string {
	switch contentType {
	case ContentTypeDocument:
		return "文档"
	case ContentTypeImage:
		return "图片"
	case ContentTypeVideo:
		return "视频"
	case ContentTypeAudio:
		return "音频"
	case ContentTypeArchive:
		return "压缩包"
	case ContentTypeCode:
		return "代码"
	case ContentTypeData:
		return "数据"
	default:
		return "其他"
	}
}

// MergeTags 合并标签，去重
func MergeTags(tagSets ...[]string) []string {
	tagMap := make(map[string]bool)
	var result []string

	for _, tags := range tagSets {
		for _, tag := range tags {
			if !tagMap[tag] {
				tagMap[tag] = true
				result = append(result, tag)
			}
		}
	}

	return result
}